package internal

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// HealthState represents the upstream connectivity state.
type HealthState string

const (
	HealthUp   HealthState = "up"
	HealthDown HealthState = "down"
)

// HealthChecker periodically probes the upstream ClinePass API with a sliding
// window. When failThreshold out of windowSize consecutive checks fail, the
// proxy is marked down. Only manual ReEnable() brings it back up.
type HealthChecker struct {
	mu            sync.RWMutex
	state         HealthState
	window        []bool // true = pass, false = fail, newest at end
	windowSize    int
	failThreshold int
	interval      time.Duration
	jitter        time.Duration
	upstreamURL   string
	client        *http.Client
	stopCh        chan struct{}
	running       bool
}

// NewHealthChecker creates a checker with the given polling interval.
// Defaults: window=5, threshold=3, jitter=±10s.
func NewHealthChecker(interval time.Duration) *HealthChecker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &HealthChecker{
		state:         HealthUp,
		window:        make([]bool, 0, 5),
		windowSize:    5,
		failThreshold: 3,
		interval:      interval,
		jitter:        10 * time.Second,
		upstreamURL:   "https://api.cline.bot/api/v1/models",
		client:        &http.Client{Timeout: 10 * time.Second},
		stopCh:        make(chan struct{}),
	}
}

// Start begins periodic health checks in a background goroutine.
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.mu.Unlock()

	go func() {
		// Immediate first check
		hc.doCheck()

		for {
			// Sleep with jitter: interval ± jitter/2
			j := time.Duration(rand.Int63n(int64(hc.jitter*2))) - hc.jitter/2
			wait := hc.interval + j
			if wait < time.Second {
				wait = time.Second
			}

			select {
			case <-time.After(wait):
				hc.doCheck()
			case <-hc.stopCh:
				return
			}
		}
	}()

	log.Printf("[HEALTH] started (interval=%v, window=%d, threshold=%d)",
		hc.interval, hc.windowSize, hc.failThreshold)
}

// Stop terminates the background checker.
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if hc.running {
		close(hc.stopCh)
		hc.running = false
	}
}

// Status returns the current health state (thread-safe).
func (hc *HealthChecker) Status() HealthState {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.state
}

// ReEnable manually resets the checker to up and clears the window.
func (hc *HealthChecker) ReEnable() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.state = HealthUp
	hc.window = make([]bool, 0, hc.windowSize)
	log.Printf("[HEALTH] manual re-enable — state reset to up")
}

// IsAvailable returns true when the proxy should accept requests.
func (hc *HealthChecker) IsAvailable() bool {
	return hc.Status() == HealthUp
}

// GetCheckInfo returns a snapshot for the admin UI and /health endpoint.
func (hc *HealthChecker) GetCheckInfo() map[string]interface{} {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	results := make([]string, len(hc.window))
	passCount := 0
	failCount := 0
	for i, r := range hc.window {
		if r {
			results[i] = "pass"
			passCount++
		} else {
			results[i] = "fail"
			failCount++
		}
	}

	return map[string]interface{}{
		"state":          string(hc.state),
		"window":         results,
		"pass_count":     passCount,
		"fail_count":     failCount,
		"window_size":    hc.windowSize,
		"fail_threshold": hc.failThreshold,
		"interval_sec":   int(hc.interval.Seconds()),
		"upstream_url":   hc.upstreamURL,
	}
}

// doCheck performs a single upstream probe and evaluates the sliding window.
func (hc *HealthChecker) doCheck() {
	pass := hc.probeUpstream()

	hc.mu.Lock()
	// Append and trim sliding window
	hc.window = append(hc.window, pass)
	if len(hc.window) > hc.windowSize {
		hc.window = hc.window[1:]
	}

	// Evaluate — only transition down when window is full
	if len(hc.window) >= hc.windowSize && hc.state == HealthUp {
		failCount := 0
		for _, r := range hc.window {
			if !r {
				failCount++
			}
		}
		if failCount >= hc.failThreshold {
			hc.state = HealthDown
			log.Printf("[HEALTH] **** DOWN **** %d/%d checks failed", failCount, hc.windowSize)
		}
	}
	state := hc.state
	hc.mu.Unlock()

	if pass {
		log.Printf("[HEALTH] check PASS (state=%s)", state)
	} else {
		log.Printf("[HEALTH] check FAIL (state=%s)", state)
	}
}

// probeUpstream tries a GET to the upstream models endpoint. Any HTTP response
// (including 4xx/5xx) counts as reachable. Only transport/connectivity errors
// count as failures.
func (hc *HealthChecker) probeUpstream() bool {
	req, err := http.NewRequest("GET", hc.upstreamURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "ClinePass-Proxy-Health/3.0")

	resp, err := hc.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	// 2xx/3xx/4xx/5xx all mean the server is reachable
	return true
}

// Must implement Stringer for clean logging.
func (hc *HealthChecker) String() string {
	info := hc.GetCheckInfo()
	return fmt.Sprintf("Health{state=%s, window=%v}", info["state"], info["window"])
}


