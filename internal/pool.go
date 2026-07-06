package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type PoolStatus string

const (
	StatusActive    PoolStatus = "active"
	StatusCooldown  PoolStatus = "cooldown"
	StatusExhausted PoolStatus = "exhausted"
	StatusMonthly   PoolStatus = "monthly"
)

// StatusLabel returns a human-readable label with remaining time
func (e *PoolEntry) StatusLabel() string {
	now := time.Now()
	switch e.Status {
	case StatusActive:
		return "available"
	case StatusCooldown:
		remain := e.CooldownUntil.Sub(now)
		if remain <= 0 {
			return "cooldown (expiring soon)"
		}
		h := int(remain.Hours())
		m := int(remain.Minutes()) % 60
		if h > 0 {
			return fmt.Sprintf("cooldown %dh%dm", h, m)
		}
		return fmt.Sprintf("cooldown %dm", m)
	case StatusExhausted:
		remain := e.WeeklyReset.Sub(now)
		if remain <= 0 {
			return "weekly quota (resetting)"
		}
		d := int(remain.Hours()) / 24
		return fmt.Sprintf("weekly quota (reset in %dd)", d)
	case StatusMonthly:
		remain := e.WeeklyReset.Sub(now)
		d := int(remain.Hours()) / 24
		return fmt.Sprintf("monthly quota (reset in %dd)", d)
	default:
		return string(e.Status)
	}
}

type PoolEntry struct {
	ID            int        `json:"id"`
	Key           string     `json:"key"`
	Status        PoolStatus `json:"status"`
	Note          string     `json:"note"`
	LastUsed      time.Time  `json:"last_used"`
	LastFailed    time.Time  `json:"last_failed"`
	FailCount     int        `json:"fail_count"`
	CooldownUntil time.Time  `json:"cooldown_until"`
	WeeklyReset   time.Time  `json:"weekly_reset"`
	CreatedAt     time.Time  `json:"created_at"`
}

type KeyPoolConfig struct {
	Keys               []*PoolEntry      `json:"keys"`
	CurrentIdx         int               `json:"current_idx"`
	CooldownHour       int               `json:"cooldown_hour"`
	MaxCooldownRetries  int              `json:"max_cooldown_retries"`
	HiddenModels       []string          `json:"hidden_models"`
	ModelOverrides     map[string]Model  `json:"model_overrides"`
	AdminPasswordHash  string             `json:"admin_password_hash"`
	mu                 sync.Mutex
	configPath         string
	dirty              bool
}

func NewKeyPool(configPath string) *KeyPoolConfig {
	pool := &KeyPoolConfig{
		CooldownHour:      5,
		MaxCooldownRetries: 3,
		configPath:        configPath,
	}
	pool.load()
	return pool
}

func (kp *KeyPoolConfig) load() {
	data, err := os.ReadFile(kp.configPath)
	if err != nil {
		log.Printf("[POOL] No config file at %s, starting fresh", kp.configPath)
		return
	}
	if err := json.Unmarshal(data, kp); err != nil {
		log.Printf("[POOL] Failed to parse config: %v", err)
		return
	}
	log.Printf("[POOL] Loaded %d keys from %s", len(kp.Keys), kp.configPath)
}

func (kp *KeyPoolConfig) Save() error {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	kp.dirty = false
	data, err := json.MarshalIndent(kp, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(kp.configPath[:len(kp.configPath)-len(kp.configPath[lastSlash(kp.configPath):])], 0755)
	return os.WriteFile(kp.configPath, data, 0644)
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return 0
}

func (kp *KeyPoolConfig) saveIfDirty() {
	if kp.dirty {
		if err := kp.Save(); err != nil {
			log.Printf("[POOL] Save error: %v", err)
		}
	}
}

// nextMonday returns the upcoming Monday 00:00
func nextMonday() time.Time {
	now := time.Now()
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7
	}
	monday := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, now.Location())
	return monday
}

// NextKey returns the next available API key from the pool.
// Returns empty string if no key is available.
func (kp *KeyPoolConfig) NextKey() string {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	defer kp.saveIfDirty()

	if len(kp.Keys) == 0 {
		return ""
	}

	now := time.Now()

	// Reset weekly exhausted keys if new week
	for _, entry := range kp.Keys {
		if entry.Status == StatusExhausted && now.After(entry.WeeklyReset) {
			entry.Status = StatusActive
			entry.FailCount = 0
			entry.CooldownUntil = time.Time{}
			log.Printf("[POOL] Key %d reset: new week", entry.ID)
			kp.dirty = true
		}
	}

	// Try up to len(keys) times
	for i := 0; i < len(kp.Keys); i++ {
		idx := (kp.CurrentIdx + i) % len(kp.Keys)
		entry := kp.Keys[idx]

		switch entry.Status {
		case StatusActive:
			kp.CurrentIdx = (idx + 1) % len(kp.Keys)
			entry.LastUsed = now
			log.Printf("[POOL] Using key %d (%s...)", entry.ID, safePrefix(entry.Key))
			return entry.Key

		case StatusCooldown:
			if now.After(entry.CooldownUntil) {
				// Cooldown expired, reset to active
				entry.Status = StatusActive
				kp.CurrentIdx = (idx + 1) % len(kp.Keys)
				entry.LastUsed = now
				log.Printf("[POOL] Key %d cooldown expired, reusing", entry.ID)
				kp.dirty = true
				return entry.Key
			}
			continue

		case StatusExhausted:
			continue
		}
	}

	log.Printf("[POOL] All keys unavailable")
	return ""
}

// MarkFailed marks a key as failed (401/402)
func (kp *KeyPoolConfig) MarkFailed(keyID int) {
	kp.mu.Lock()
	defer kp.mu.Unlock()
	defer kp.saveIfDirty()

	for _, entry := range kp.Keys {
		if entry.ID == keyID {
			now := time.Now()
			entry.LastFailed = now
			entry.FailCount++
			duration := time.Duration(kp.CooldownHour) * time.Hour

			if entry.FailCount >= kp.MaxCooldownRetries {
				entry.Status = StatusExhausted
				entry.WeeklyReset = nextMonday()
				log.Printf("[POOL] Key %d exhausted after %d failures, reset at %s",
					keyID, entry.FailCount, entry.WeeklyReset.Format("2006-01-02"))
			} else {
				entry.Status = StatusCooldown
				entry.CooldownUntil = now.Add(duration)
				log.Printf("[POOL] Key %d cooling down %dh until %s (fail %d/%d)",
					keyID, kp.CooldownHour, entry.CooldownUntil.Format("15:04"),
					entry.FailCount, kp.MaxCooldownRetries)
			}
			kp.dirty = true
			return
		}
	}
}

// MarkSuccess marks a key as successfully used
func (kp *KeyPoolConfig) MarkSuccess(keyID int) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for _, entry := range kp.Keys {
		if entry.ID == keyID {
			entry.LastUsed = time.Now()
			if entry.Status == StatusCooldown {
				entry.Status = StatusActive
				entry.FailCount = 0
				entry.CooldownUntil = time.Time{}
				log.Printf("[POOL] Key %d recovered from cooldown", keyID)
				kp.dirty = true
			}
			return
		}
	}
}

// FindKeyByValue finds a key entry by its API key string
func (kp *KeyPoolConfig) FindKeyByValue(key string) *PoolEntry {
	for _, entry := range kp.Keys {
		if entry.Key == key {
			return entry
		}
	}
	return nil
}

// ============================================================
// Model management helpers (persisted alongside pool config)
// ============================================================

func (kp *KeyPoolConfig) IsModelHidden(id string) bool {
	for _, h := range kp.HiddenModels {
		if h == id {
			return true
		}
	}
	return false
}

func (kp *KeyPoolConfig) HideModel(id string) {
	if kp.IsModelHidden(id) {
		return
	}
	kp.HiddenModels = append(kp.HiddenModels, id)
	kp.dirty = true
	kp.saveIfDirty()
}

func (kp *KeyPoolConfig) UnhideModel(id string) {
	for i, h := range kp.HiddenModels {
		if h == id {
			kp.HiddenModels = append(kp.HiddenModels[:i], kp.HiddenModels[i+1:]...)
			kp.dirty = true
			kp.saveIfDirty()
			return
		}
	}
}

func (kp *KeyPoolConfig) GetModelOverride(id string) (Model, bool) {
	m, ok := kp.ModelOverrides[id]
	return m, ok
}

func (kp *KeyPoolConfig) SetModelOverride(id string, m Model) {
	if kp.ModelOverrides == nil {
		kp.ModelOverrides = make(map[string]Model)
	}
	kp.ModelOverrides[id] = m
	kp.dirty = true
	kp.saveIfDirty()
}

func (kp *KeyPoolConfig) RemoveModelOverride(id string) {
	delete(kp.ModelOverrides, id)
	kp.dirty = true
	kp.saveIfDirty()
}

// AddKey adds a new key to the pool
func (kp *KeyPoolConfig) AddKey(key, note string) *PoolEntry {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	maxID := 0
	for _, entry := range kp.Keys {
		if entry.ID > maxID {
			maxID = entry.ID
		}
	}

	entry := &PoolEntry{
		ID:        maxID + 1,
		Key:       key,
		Status:    StatusActive,
		Note:      note,
		CreatedAt: time.Now(),
	}
	kp.Keys = append(kp.Keys, entry)
	kp.dirty = true
	log.Printf("[POOL] Added key %d", entry.ID)
	return entry
}

// RemoveKey removes a key by ID
func (kp *KeyPoolConfig) RemoveKey(id int) bool {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for i, entry := range kp.Keys {
		if entry.ID == id {
			kp.Keys = append(kp.Keys[:i], kp.Keys[i+1:]...)
			kp.dirty = true
			log.Printf("[POOL] Removed key %d", id)
			return true
		}
	}
	return false
}

// TestKey tests a single key against the ClinePass API
func (kp *KeyPoolConfig) TestKey(key string) (bool, string) {
	// Simple test: make a minimal chat completion request
	body := fmt.Sprintf(`{"model":"cline-pass/deepseek-v4-flash","messages":[{"role":"user","content":"hi"}],"stream":false,"max_tokens":1}`)
	req, err := http.NewRequest("POST", clinepassBaseURL+"/chat/completions",
		bytes.NewReader([]byte(body)))
	if err != nil {
		return false, "Request error: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "Connection error: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true, "OK"
	}
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	return false, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(errBody))
}

// TestAllKeys tests all keys and updates their status
func (kp *KeyPoolConfig) TestAllKeys() []map[string]interface{} {
	results := []map[string]interface{}{}
	for _, entry := range kp.Keys {
		ok, msg := kp.TestKey(entry.Key)
		result := map[string]interface{}{
			"id":     entry.ID,
			"key":    safePrefix(entry.Key),
			"status": entry.Status,
			"ok":     ok,
			"msg":    msg,
		}
		results = append(results, result)
		log.Printf("[POOL] Test key %d (%s...): %v - %s", entry.ID, safePrefix(entry.Key), ok, msg)
	}
	return results
}

func safePrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "..."
}
