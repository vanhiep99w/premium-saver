package admin

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const sessionExpiry = 30 * 24 * time.Hour // 1 month

type sessionInfo struct {
	expiresAt time.Time
	csrfToken string
}

// SessionManager manages admin sessions in memory.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]sessionInfo
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]sessionInfo),
	}
}

// CreateSession creates a new session and returns the session token.
func (sm *SessionManager) CreateSession() (token string, csrf string, err error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", "", err
	}
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		return "", "", err
	}

	token = hex.EncodeToString(tokenBytes)
	csrf = hex.EncodeToString(csrfBytes)

	sm.mu.Lock()
	sm.sessions[token] = sessionInfo{
		expiresAt: time.Now().Add(sessionExpiry),
		csrfToken: csrf,
	}
	sm.mu.Unlock()

	return token, csrf, nil
}

// ValidateSession checks if a session token is valid. Returns the CSRF token if valid.
// Lazily evicts expired sessions.
func (sm *SessionManager) ValidateSession(token string) (csrfToken string, valid bool) {
	sm.mu.RLock()
	sess, exists := sm.sessions[token]
	sm.mu.RUnlock()

	if !exists {
		return "", false
	}

	if time.Now().After(sess.expiresAt) {
		sm.mu.Lock()
		delete(sm.sessions, token)
		sm.mu.Unlock()
		return "", false
	}

	return sess.csrfToken, true
}

// DeleteSession removes a session.
func (sm *SessionManager) DeleteSession(token string) {
	sm.mu.Lock()
	delete(sm.sessions, token)
	sm.mu.Unlock()
}

// SetSessionCookie sets the session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(sessionExpiry.Seconds()),
	})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

// GetSessionToken extracts the session token from the request cookie.
func GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}
