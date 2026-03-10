package auth

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/hieptran/copilot-proxy/config"
)

// TokenData holds the persisted authentication state.
type TokenData struct {
	// OAuthToken is the long-lived GitHub OAuth access token (used to refresh Copilot tokens).
	OAuthToken string `json:"oauth_token"`
	// CopilotToken is the short-lived Copilot API token.
	CopilotToken string `json:"copilot_token"`
	// ExpiresAt is the Unix timestamp (milliseconds) when the Copilot token expires.
	ExpiresAt int64 `json:"expires_at"`
}

// Store manages reading and writing auth tokens to disk.
type Store struct {
	mu   sync.RWMutex
	data TokenData
	path string
}

// NewStore creates a new Store, loading existing tokens from disk if available.
func NewStore() (*Store, error) {
	path, err := config.AuthFilePath()
	if err != nil {
		return nil, err
	}

	s := &Store{path: path}
	_ = s.load() // Ignore error if file doesn't exist yet
	return s, nil
}

// load reads token data from the file.
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.data)
}

// save writes token data to disk.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// Get returns a copy of the current token data.
func (s *Store) Get() TokenData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// SetOAuthToken stores the OAuth token and persists to disk.
func (s *Store) SetOAuthToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.OAuthToken = token
	return s.save()
}

// SetCopilotToken stores the Copilot API token and its expiry, then persists to disk.
func (s *Store) SetCopilotToken(token string, expiresAt int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.CopilotToken = token
	s.data.ExpiresAt = expiresAt
	return s.save()
}

// Clear removes all stored tokens and deletes the auth file.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = TokenData{}
	return os.Remove(s.path)
}

// HasOAuthToken returns true if an OAuth token is stored.
func (s *Store) HasOAuthToken() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.OAuthToken != ""
}
