package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
)

// ============================================================
// Admin Auth - password-based session management
// ============================================================

const (
	defaultPassword = "123456"
	sessionSalt     = "clinepass-admin-session-v1"
)

// SetAdminPassword hashes and stores the admin password
func (kp *KeyPoolConfig) SetAdminPassword(password string) {
	hash := sha256.Sum256([]byte(password))
	kp.AdminPasswordHash = hex.EncodeToString(hash[:])
	kp.dirty = true
	kp.saveIfDirty()
	log.Printf("[AUTH] Admin password updated")
}

// CheckPassword validates a password against the stored hash
func (kp *KeyPoolConfig) CheckPassword(password string) bool {
	if kp.AdminPasswordHash == "" {
		// No password set = use default
		return password == defaultPassword
	}
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:]) == kp.AdminPasswordHash
}

// GenerateSessionToken creates a stateless session token from the password hash
func (kp *KeyPoolConfig) GenerateSessionToken() string {
	data := kp.AdminPasswordHash
	if data == "" {
		defaultHash := sha256.Sum256([]byte(defaultPassword))
		data = hex.EncodeToString(defaultHash[:])
	}
	tokenData := sha256.Sum256([]byte(data + ":" + sessionSalt))
	return hex.EncodeToString(tokenData[:])
}

// ValidateSessionToken checks if a session token is valid
func (kp *KeyPoolConfig) ValidateSessionToken(token string) bool {
	if token == "" {
		return false
	}
	return token == kp.GenerateSessionToken()
}

// IsPasswordDefault returns true if no password has been changed from default
func (kp *KeyPoolConfig) IsPasswordDefault() bool {
	if kp.AdminPasswordHash == "" {
		return true
	}
	defaultHash := sha256.Sum256([]byte(defaultPassword))
	return kp.AdminPasswordHash == hex.EncodeToString(defaultHash[:])
}

func (kp *KeyPoolConfig) PasswordStatusLabel() string {
	if kp.IsPasswordDefault() {
		return fmt.Sprintf("default (change recommended)")
	}
	return "custom"
}
