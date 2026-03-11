# User Management & Usage Reporting Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-user API key authentication, admin UI, and per-user usage reporting to the copilot-proxy.

**Architecture:** Monolith — single binary, single port. New packages: `db/` (SQLite), `admin/` (HTTP handlers + session), `web/` (embedded templates + static). Proxy validates API keys via SHA-256 hash lookup, tracks token usage by intercepting SSE stream responses through a pipe-through reader in `ModifyResponse`.

**Tech Stack:** Go 1.23.6, SQLite (modernc.org/sqlite), Go html/template, vanilla JS, embed.FS

**Spec:** `docs/superpowers/specs/2026-03-11-user-management-reporting-design.md`

---

## Chunk 1: Foundation — Database, Config, Dependencies

### Task 1: Add dependencies and update config

**Files:**
- Modify: `go.mod`
- Modify: `config/config.go`

- [ ] **Step 1: Add new dependencies**

```bash
cd /home/hieptran/Desktop/premium-saver
go get modernc.org/sqlite
go get github.com/google/uuid
go get golang.org/x/crypto
```

- [ ] **Step 2: Add new config constants and functions to `config/config.go`**

Add after the existing `AuthFilePath()` function:

```go
// DBPath returns the SQLite database file path.
// Uses DB_PATH env var, or defaults to ~/.config/copilot-proxy/copilot-proxy.db
// Note: "os" is already imported in this file, no need to add it again.
func DBPath() (string, error) {
	if p := os.Getenv("DB_PATH"); p != "" {
		dir := filepath.Dir(p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", err
		}
		return p, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configDir, "copilot-proxy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "copilot-proxy.db"), nil
}

// AdminUsername returns the admin username from ADMIN_USERNAME env var (default: "admin").
func AdminUsername() string {
	if u := os.Getenv("ADMIN_USERNAME"); u != "" {
		return u
	}
	return "admin"
}

// AdminPassword returns the admin password from ADMIN_PASSWORD env var.
// Returns empty string if not set.
func AdminPassword() string {
	return os.Getenv("ADMIN_PASSWORD")
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum config/config.go
git commit -m "feat: add SQLite and UUID dependencies, add DB/admin config"
```

---

### Task 2: Create database package — init and migrations

**Files:**
- Create: `db/db.go`

- [ ] **Step 1: Write `db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB connection to the SQLite database.
type DB struct {
	conn *sql.DB
}

// New opens the SQLite database at the given path, runs migrations, and returns a DB.
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(1) // SQLite supports one writer at a time

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for use in other packages.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS admin (
		id            INTEGER PRIMARY KEY CHECK (id = 1),
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		name           TEXT NOT NULL,
		api_key_hash   TEXT NOT NULL UNIQUE,
		api_key_prefix TEXT NOT NULL,
		active         BOOLEAN DEFAULT 1,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS usage_logs (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		model             TEXT,
		path              TEXT,
		prompt_tokens     INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		total_tokens      INTEGER DEFAULT 0,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_usage_logs_user_time ON usage_logs(user_id, created_at);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// StartCleanupJob starts a goroutine that deletes usage_logs older than 30 days every hour.
func (db *DB) StartCleanupJob() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			result, err := db.conn.Exec("DELETE FROM usage_logs WHERE created_at < datetime('now', '-30 days')")
			if err != nil {
				log.Printf("ERROR: cleanup job failed: %v", err)
				continue
			}
			if rows, _ := result.RowsAffected(); rows > 0 {
				log.Printf("Cleanup: deleted %d old usage logs", rows)
			}
		}
	}()
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./db/...
```

Expected: compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add db/db.go
git commit -m "feat: add database package with SQLite init and migrations"
```

---

### Task 3: Create user CRUD operations

**Files:**
- Create: `db/users.go`

- [ ] **Step 1: Write `db/users.go`**

```go
package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a proxy user.
type User struct {
	ID           int
	Name         string
	APIKeyPrefix string
	Active       bool
	CreatedAt    time.Time
}

// CreateUser creates a new user and returns the user and the plaintext API key (shown once).
func (db *DB) CreateUser(name string) (*User, string, error) {
	apiKey := "sk-" + uuid.New().String()
	hash := hashAPIKey(apiKey)
	prefix := apiKey[:11] // e.g., "sk-550e8400"

	result, err := db.conn.Exec(
		"INSERT INTO users (name, api_key_hash, api_key_prefix, active) VALUES (?, ?, ?, 1)",
		name, hash, prefix,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	id, _ := result.LastInsertId()
	user := &User{
		ID:           int(id),
		Name:         name,
		APIKeyPrefix: prefix,
		Active:       true,
		CreatedAt:    time.Now(),
	}
	return user, apiKey, nil
}

// GetUserByAPIKey looks up a user by their API key (hashed).
// Returns nil if not found.
func (db *DB) GetUserByAPIKey(apiKey string) (*User, error) {
	hash := hashAPIKey(apiKey)
	var u User
	var active int
	err := db.conn.QueryRow(
		"SELECT id, name, api_key_prefix, active, created_at FROM users WHERE api_key_hash = ?",
		hash,
	).Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

// ListUsers returns all users.
func (db *DB) ListUsers() ([]User, error) {
	rows, err := db.conn.Query(
		"SELECT id, name, api_key_prefix, active, created_at FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var active int
		if err := rows.Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Active = active == 1
		users = append(users, u)
	}
	return users, rows.Err()
}

// DeleteUser deletes a user by ID (cascade deletes usage_logs).
func (db *DB) DeleteUser(id int) error {
	result, err := db.conn.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// SetUserActive sets the active status of a user.
func (db *DB) SetUserActive(id int, active bool) error {
	val := 0
	if active {
		val = 1
	}
	result, err := db.conn.Exec("UPDATE users SET active = ? WHERE id = ?", val, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetUser returns a single user by ID.
func (db *DB) GetUser(id int) (*User, error) {
	var u User
	var active int
	err := db.conn.QueryRow(
		"SELECT id, name, api_key_prefix, active, created_at FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Name, &u.APIKeyPrefix, &active, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	u.Active = active == 1
	return &u, nil
}

func hashAPIKey(apiKey string) string {
	h := sha256.Sum256([]byte(apiKey))
	return fmt.Sprintf("%x", h)
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./db/...
```

- [ ] **Step 3: Commit**

```bash
git add db/users.go
git commit -m "feat: add user CRUD with SHA-256 API key hashing"
```

---

### Task 4: Create usage stats operations

**Files:**
- Create: `db/usage.go`

- [ ] **Step 1: Write `db/usage.go`**

```go
package db

import "time"

// UsageRecord represents a single API request's usage data.
type UsageRecord struct {
	UserID           int
	Model            string
	Path             string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// UsageStats holds aggregated usage data for a time period.
type UsageStats struct {
	Period           string
	RequestCount     int
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
}

// RecentRequest represents a single recent request for display.
type RecentRequest struct {
	Time             time.Time
	Path             string
	Model            string
	TotalTokens      int
}

// InsertUsage inserts a usage log record.
func (db *DB) InsertUsage(r UsageRecord) error {
	_, err := db.conn.Exec(
		`INSERT INTO usage_logs (user_id, model, path, prompt_tokens, completion_tokens, total_tokens)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.UserID, r.Model, r.Path, r.PromptTokens, r.CompletionTokens, r.TotalTokens,
	)
	return err
}

// GetUsageStats returns aggregated usage stats for a user across multiple time periods.
func (db *DB) GetUsageStats(userID int) ([]UsageStats, error) {
	periods := []struct {
		name     string
		duration time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"5h", 5 * time.Hour},
		{"1d", 24 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
	}

	var stats []UsageStats
	for _, p := range periods {
		since := time.Now().Add(-p.duration)
		var s UsageStats
		s.Period = p.name
		err := db.conn.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0)
			 FROM usage_logs WHERE user_id = ? AND created_at >= ?`,
			userID, since,
		).Scan(&s.RequestCount, &s.PromptTokens, &s.CompletionTokens, &s.TotalTokens)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetRecentRequests returns the last N requests for a user.
func (db *DB) GetRecentRequests(userID int, limit int) ([]RecentRequest, error) {
	rows, err := db.conn.Query(
		`SELECT created_at, path, model, total_tokens FROM usage_logs
		 WHERE user_id = ? ORDER BY created_at DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reqs []RecentRequest
	for rows.Next() {
		var r RecentRequest
		if err := rows.Scan(&r.Time, &r.Path, &r.Model, &r.TotalTokens); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// GetRequestCount24h returns the request count in the last 24 hours for a user.
func (db *DB) GetRequestCount24h(userID int) (int, error) {
	var count int
	err := db.conn.QueryRow(
		"SELECT COUNT(*) FROM usage_logs WHERE user_id = ? AND created_at >= datetime('now', '-1 day')",
		userID,
	).Scan(&count)
	return count, err
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./db/...
```

- [ ] **Step 3: Commit**

```bash
git add db/usage.go
git commit -m "feat: add usage stats queries and insert operations"
```

---

## Chunk 2: Admin Authentication & Session Management

### Task 5: Create session management

**Files:**
- Create: `admin/session.go`

- [ ] **Step 1: Write `admin/session.go`**

```go
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
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./admin/...
```

- [ ] **Step 3: Commit**

```bash
git add admin/session.go
git commit -m "feat: add admin session management with CSRF tokens"
```

---

### Task 6: Create admin handlers — login, logout, user CRUD, reports

**Files:**
- Create: `admin/handlers.go`

- [ ] **Step 1: Write `admin/handlers.go`**

```go
package admin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hieptran/copilot-proxy/db"
	"golang.org/x/crypto/bcrypt"
)

// Admin handles admin UI routes.
type Admin struct {
	db       *db.DB
	sessions *SessionManager
	tmpl     *template.Template
	throttle *loginThrottle
}

type loginThrottle struct {
	mu       sync.Mutex
	failures map[string]throttleState
}

type throttleState struct {
	count     int
	lastFail  time.Time
}

// New creates a new Admin handler.
func New(database *db.DB, tmpl *template.Template) *Admin {
	return &Admin{
		db:       database,
		sessions: NewSessionManager(),
		tmpl:     tmpl,
		throttle: &loginThrottle{failures: make(map[string]throttleState)},
	}
}

// SetupAdmin seeds the admin account from env vars if needed.
func (a *Admin) SetupAdmin(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Upsert: insert or update if password changed
	_, err = a.db.Conn().Exec(
		`INSERT INTO admin (id, username, password_hash) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET username = excluded.username, password_hash = excluded.password_hash`,
		username, string(hash),
	)
	return err
}

// RegisterRoutes registers all admin routes on the given mux.
func (a *Admin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/login", a.handleLoginPage)
	mux.HandleFunc("POST /admin/login", a.handleLogin)
	mux.HandleFunc("POST /admin/logout", a.requireAuth(a.handleLogout))
	mux.HandleFunc("GET /admin/", a.requireAuth(a.handleDashboard))
	mux.HandleFunc("GET /admin/users", a.requireAuth(a.handleUsersPage))
	mux.HandleFunc("POST /admin/users", a.requireAuth(a.handleCreateUser))
	mux.HandleFunc("DELETE /admin/users/{id}", a.requireAuth(a.handleDeleteUser))
	mux.HandleFunc("PATCH /admin/users/{id}", a.requireAuth(a.handleUpdateUser))
	mux.HandleFunc("GET /admin/report/{id}", a.requireAuth(a.handleReportPage))
	mux.HandleFunc("GET /admin/api/report/{id}", a.requireAuth(a.handleReportAPI))
}

// requireAuth wraps a handler with session authentication and CSRF validation.
func (a *Admin) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := GetSessionToken(r)
		csrf, valid := a.sessions.ValidateSession(token)
		if !valid {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		// CSRF check for state-changing methods
		if r.Method == "POST" || r.Method == "DELETE" || r.Method == "PATCH" {
			reqCSRF := r.Header.Get("X-CSRF-Token")
			if reqCSRF == "" {
				reqCSRF = r.FormValue("csrf_token")
			}
			if reqCSRF != csrf {
				http.Error(w, `{"error": "invalid CSRF token"}`, http.StatusForbidden)
				return
			}
		}

		// Store CSRF token in request context via header for templates
		r.Header.Set("X-CSRF-Token", csrf)
		next(w, r)
	}
}

func (a *Admin) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	// If already logged in, redirect
	token := GetSessionToken(r)
	if _, valid := a.sessions.ValidateSession(token); valid {
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	a.tmpl.ExecuteTemplate(w, "login.html", map[string]any{
		"Error": r.URL.Query().Get("error"),
	})
}

func (a *Admin) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	// Check throttle
	a.throttle.mu.Lock()
	state := a.throttle.failures[ip]
	if state.count >= 3 {
		delay := time.Duration(1<<min(state.count-3, 4)) * time.Second // 1s, 2s, 4s, 8s, 16s, max 30s
		if delay > 30*time.Second {
			delay = 30 * time.Second
		}
		if time.Since(state.lastFail) < delay {
			a.throttle.mu.Unlock()
			http.Redirect(w, r, "/admin/login?error=Too+many+attempts,+try+again+later", http.StatusSeeOther)
			return
		}
		// Auto-expire after 15 minutes
		if time.Since(state.lastFail) > 15*time.Minute {
			delete(a.throttle.failures, ip)
			state = throttleState{}
		}
	}
	a.throttle.mu.Unlock()

	username := r.FormValue("username")
	password := r.FormValue("password")

	var storedHash string
	var storedUsername string
	err := a.db.Conn().QueryRow("SELECT username, password_hash FROM admin WHERE id = 1").Scan(&storedUsername, &storedHash)
	if err != nil || username != storedUsername || bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) != nil {
		a.throttle.mu.Lock()
		state.count++
		state.lastFail = time.Now()
		a.throttle.failures[ip] = state
		a.throttle.mu.Unlock()

		http.Redirect(w, r, "/admin/login?error=Invalid+credentials", http.StatusSeeOther)
		return
	}

	// Successful login — clear throttle
	a.throttle.mu.Lock()
	delete(a.throttle.failures, ip)
	a.throttle.mu.Unlock()

	token, csrf, err := a.sessions.CreateSession()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	_ = csrf
	SetSessionCookie(w, token)
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (a *Admin) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := GetSessionToken(r)
	a.sessions.DeleteSession(token)
	ClearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (a *Admin) handleDashboard(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (a *Admin) handleUsersPage(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Get 24h request count for each user
	type userWithStats struct {
		db.User
		Requests24h int
	}
	var usersWithStats []userWithStats
	for _, u := range users {
		count, _ := a.db.GetRequestCount24h(u.ID)
		usersWithStats = append(usersWithStats, userWithStats{User: u, Requests24h: count})
	}

	csrf := r.Header.Get("X-CSRF-Token")
	a.tmpl.ExecuteTemplate(w, "users.html", map[string]any{
		"Users":     usersWithStats,
		"CSRFToken": csrf,
	})
}

func (a *Admin) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, `{"error": "name is required"}`, http.StatusBadRequest)
		return
	}

	user, apiKey, err := a.db.CreateUser(name)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user":    user,
		"api_key": apiKey,
	})
}

func (a *Admin) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error": "invalid id"}`, http.StatusBadRequest)
		return
	}

	if err := a.db.DeleteUser(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok": true}`))
}

func (a *Admin) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error": "invalid id"}`, http.StatusBadRequest)
		return
	}

	var body struct {
		Active *bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Active == nil {
		http.Error(w, `{"error": "body must contain {\"active\": true/false}"}`, http.StatusBadRequest)
		return
	}

	if err := a.db.SetUserActive(id, *body.Active); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok": true}`))
}

func (a *Admin) handleReportPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := a.db.GetUser(id)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	stats, err := a.db.GetUsageStats(id)
	if err != nil {
		log.Printf("ERROR: failed to get usage stats: %v", err)
	}

	recentReqs, err := a.db.GetRecentRequests(id, 50)
	if err != nil {
		log.Printf("ERROR: failed to get recent requests: %v", err)
	}

	csrf := r.Header.Get("X-CSRF-Token")
	a.tmpl.ExecuteTemplate(w, "report.html", map[string]any{
		"User":           user,
		"Stats":          stats,
		"RecentRequests": recentReqs,
		"CSRFToken":      csrf,
	})
}

func (a *Admin) handleReportAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error": "invalid id"}`, http.StatusBadRequest)
		return
	}

	stats, err := a.db.GetUsageStats(id)
	if err != nil {
		http.Error(w, `{"error": "internal error"}`, http.StatusInternalServerError)
		return
	}

	recentReqs, err := a.db.GetRecentRequests(id, 50)
	if err != nil {
		http.Error(w, `{"error": "internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"stats":           stats,
		"recent_requests": recentReqs,
	})
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./admin/...
```

- [ ] **Step 3: Commit**

```bash
git add admin/handlers.go go.mod go.sum
git commit -m "feat: add admin handlers — login, logout, user CRUD, reports"
```

---

## Chunk 3: Web Templates & Static Assets

### Task 7: Create HTML templates and static assets

**Files:**
- Create: `web/embed.go`
- Create: `web/templates/layout.html`
- Create: `web/templates/login.html`
- Create: `web/templates/users.html`
- Create: `web/templates/report.html`
- Create: `web/static/style.css`
- Create: `web/static/app.js`

- [ ] **Step 1: Write `web/embed.go`**

```go
package web

import "embed"

//go:embed templates/*.html
var Templates embed.FS

//go:embed static/*
var Static embed.FS
```

- [ ] **Step 2: Write `web/templates/layout.html`**

```html
{{define "layout"}}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="csrf-token" content="{{.CSRFToken}}">
    <title>Copilot Proxy Admin</title>
    <link rel="stylesheet" href="/admin/static/style.css">
</head>
<body>
    <div class="container">
        <header>
            <div class="header-left">
                <h1>Copilot Proxy Admin</h1>
            </div>
            <nav>
                <a href="/admin/users" class="nav-link active">Users</a>
                <form method="POST" action="/admin/logout" style="display:inline;">
                    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
                    <button type="submit" class="btn-logout">Logout</button>
                </form>
            </nav>
        </header>
        <main>
            {{template "content" .}}
        </main>
    </div>
    <script src="/admin/static/app.js"></script>
</body>
</html>
{{end}}
```

- [ ] **Step 3: Write `web/templates/login.html`**

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Copilot Proxy Admin</title>
    <link rel="stylesheet" href="/admin/static/style.css">
</head>
<body>
    <div class="login-container">
        <h1>Copilot Proxy</h1>
        <p class="subtitle">Admin Login</p>
        {{if .Error}}
        <div class="error">{{.Error}}</div>
        {{end}}
        <form method="POST" action="/admin/login">
            <input type="text" name="username" placeholder="Username" required autofocus>
            <input type="password" name="password" placeholder="Password" required>
            <button type="submit" class="btn-primary">Login</button>
        </form>
    </div>
</body>
</html>
```

- [ ] **Step 4: Write `web/templates/users.html`**

```html
{{define "content"}}
<div class="page-header">
    <h2>User Management</h2>
    <button class="btn-primary" onclick="showAddUser()">+ Add User</button>
</div>

<div id="add-user-modal" class="modal" style="display:none;">
    <div class="modal-content">
        <h3>Add New User</h3>
        <input type="text" id="new-user-name" placeholder="User name" autofocus>
        <div class="modal-actions">
            <button class="btn-secondary" onclick="hideAddUser()">Cancel</button>
            <button class="btn-primary" onclick="createUser()">Create</button>
        </div>
    </div>
</div>

<div id="api-key-modal" class="modal" style="display:none;">
    <div class="modal-content">
        <h3>API Key Created</h3>
        <p class="warning">Copy this API key now. It cannot be retrieved again.</p>
        <div class="api-key-display">
            <code id="api-key-value"></code>
            <button class="btn-copy" onclick="copyAPIKey()">Copy</button>
        </div>
        <button class="btn-primary" onclick="hideAPIKey()">Done</button>
    </div>
</div>

<table class="data-table">
    <thead>
        <tr>
            <th>Name</th>
            <th>API Key</th>
            <th>Status</th>
            <th>Requests (24h)</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody id="users-table">
        {{range .Users}}
        <tr id="user-row-{{.ID}}">
            <td>{{.Name}}</td>
            <td><code class="key-prefix">{{.APIKeyPrefix}}...</code></td>
            <td>
                <span class="status-badge {{if .Active}}active{{else}}inactive{{end}}"
                      onclick="toggleActive({{.ID}}, {{.Active}})">
                    {{if .Active}}active{{else}}inactive{{end}}
                </span>
            </td>
            <td>{{.Requests24h}}</td>
            <td class="actions">
                <a href="/admin/report/{{.ID}}" class="btn-link">report</a>
                <button class="btn-danger-sm" onclick="deleteUser({{.ID}}, '{{.Name}}')">delete</button>
            </td>
        </tr>
        {{else}}
        <tr><td colspan="5" class="empty">No users yet. Click "Add User" to create one.</td></tr>
        {{end}}
    </tbody>
</table>
{{end}}

{{template "layout" .}}
```

- [ ] **Step 5: Write `web/templates/report.html`**

```html
{{define "content"}}
<div class="page-header">
    <div>
        <a href="/admin/users" class="back-link">&larr; Back to Users</a>
        <h2>Report: {{.User.Name}}
            <span class="status-badge {{if .User.Active}}active{{else}}inactive{{end}}">
                {{if .User.Active}}active{{else}}inactive{{end}}
            </span>
        </h2>
    </div>
</div>

<section class="stats-section">
    <h3>Request Count</h3>
    <div class="stats-cards">
        {{range .Stats}}
        <div class="stat-card">
            <div class="stat-label">{{.Period}}</div>
            <div class="stat-value request-count">{{.RequestCount}}</div>
        </div>
        {{end}}
    </div>
</section>

<section class="stats-section">
    <h3>Token Usage</h3>
    <div class="stats-cards">
        {{range .Stats}}
        <div class="stat-card">
            <div class="stat-label">{{.Period}}</div>
            <div class="stat-value token-count">{{.TotalTokens}}</div>
            <div class="stat-detail">P:{{.PromptTokens}} C:{{.CompletionTokens}}</div>
        </div>
        {{end}}
    </div>
</section>

<section class="stats-section">
    <h3>Recent Requests</h3>
    <table class="data-table">
        <thead>
            <tr>
                <th>Time</th>
                <th>Path</th>
                <th>Model</th>
                <th>Tokens</th>
            </tr>
        </thead>
        <tbody>
            {{range .RecentRequests}}
            <tr>
                <td class="time-ago" data-time="{{.Time.Format "2006-01-02T15:04:05Z"}}">{{.Time.Format "15:04:05"}}</td>
                <td><code>{{.Path}}</code></td>
                <td>{{.Model}}</td>
                <td>{{.TotalTokens}}</td>
            </tr>
            {{else}}
            <tr><td colspan="4" class="empty">No requests yet.</td></tr>
            {{end}}
        </tbody>
    </table>
</section>
{{end}}

{{template "layout" .}}
```

- [ ] **Step 6: Write `web/static/style.css`**

```css
* { margin: 0; padding: 0; box-sizing: border-box; }

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace;
    background: #1a1a2e;
    color: #e0e0e0;
    min-height: 100vh;
}

.container { max-width: 960px; margin: 0 auto; padding: 20px; }

/* Header */
header {
    display: flex; justify-content: space-between; align-items: center;
    background: #16213e; padding: 12px 20px; border-radius: 8px; margin-bottom: 24px;
}
header h1 { color: #00d4ff; font-size: 18px; }
nav { display: flex; gap: 12px; align-items: center; }
.nav-link {
    color: #fff; text-decoration: none; padding: 4px 8px;
    border-bottom: 2px solid #00d4ff;
}
.btn-logout {
    background: #e94560; color: #fff; border: none; padding: 6px 14px;
    border-radius: 4px; cursor: pointer; font-size: 13px;
}
.btn-logout:hover { background: #c0392b; }

/* Login */
.login-container {
    max-width: 360px; margin: 120px auto; text-align: center;
    background: #16213e; padding: 40px; border-radius: 12px;
}
.login-container h1 { color: #00d4ff; margin-bottom: 8px; }
.login-container .subtitle { color: #888; margin-bottom: 24px; }
.login-container input {
    width: 100%; padding: 10px 14px; margin-bottom: 12px; border: 1px solid #333;
    border-radius: 6px; background: #1a1a2e; color: #fff; font-size: 14px;
}
.login-container input:focus { border-color: #00d4ff; outline: none; }
.error { color: #e94560; margin-bottom: 12px; font-size: 14px; }

/* Buttons */
.btn-primary {
    background: #4ade80; color: #000; border: none; padding: 10px 20px;
    border-radius: 6px; cursor: pointer; font-size: 14px; font-weight: 600;
}
.btn-primary:hover { background: #22c55e; }
.btn-secondary {
    background: #333; color: #fff; border: none; padding: 10px 20px;
    border-radius: 6px; cursor: pointer; font-size: 14px;
}
.btn-danger-sm {
    background: none; color: #e94560; border: none; cursor: pointer; font-size: 13px;
}
.btn-danger-sm:hover { text-decoration: underline; }
.btn-link { color: #60a5fa; text-decoration: none; font-size: 13px; }
.btn-link:hover { text-decoration: underline; }
.btn-copy {
    background: #333; color: #fff; border: none; padding: 6px 12px;
    border-radius: 4px; cursor: pointer; font-size: 12px;
}

/* Page header */
.page-header {
    display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;
}
.page-header h2 { font-size: 20px; }
.back-link { color: #60a5fa; text-decoration: none; font-size: 13px; display: block; margin-bottom: 4px; }

/* Table */
.data-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.data-table th {
    text-align: left; color: #888; padding: 10px 12px;
    border-bottom: 1px solid #333; font-weight: normal; font-size: 13px;
}
.data-table td { padding: 10px 12px; border-bottom: 1px solid #222; }
.data-table tbody tr:hover { background: #16213e; }
.key-prefix { color: #888; font-size: 12px; }
.actions { display: flex; gap: 12px; }
.empty { color: #555; text-align: center; padding: 40px !important; }

/* Status badge */
.status-badge {
    display: inline-block; padding: 2px 8px; border-radius: 3px;
    font-size: 12px; cursor: pointer;
}
.status-badge.active { background: #4ade80; color: #000; }
.status-badge.inactive { background: #ef4444; color: #fff; }

/* Modal */
.modal {
    position: fixed; top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.7); display: flex; align-items: center; justify-content: center;
    z-index: 100;
}
.modal-content {
    background: #16213e; padding: 28px; border-radius: 12px;
    min-width: 400px; max-width: 500px;
}
.modal-content h3 { margin-bottom: 16px; }
.modal-content input {
    width: 100%; padding: 10px 14px; margin-bottom: 16px; border: 1px solid #333;
    border-radius: 6px; background: #1a1a2e; color: #fff; font-size: 14px;
}
.modal-content input:focus { border-color: #00d4ff; outline: none; }
.modal-actions { display: flex; gap: 8px; justify-content: flex-end; }
.warning { color: #fbbf24; margin-bottom: 12px; font-size: 13px; }
.api-key-display {
    background: #1a1a2e; padding: 12px; border-radius: 6px; margin-bottom: 16px;
    display: flex; justify-content: space-between; align-items: center; gap: 8px;
    word-break: break-all;
}
.api-key-display code { color: #4ade80; font-size: 13px; flex: 1; }

/* Stats */
.stats-section { margin-bottom: 28px; }
.stats-section h3 { font-size: 15px; color: #ccc; margin-bottom: 10px; }
.stats-cards { display: flex; gap: 10px; }
.stat-card {
    background: #0f3460; padding: 14px; border-radius: 8px; flex: 1; text-align: center;
}
.stat-label { color: #888; font-size: 12px; margin-bottom: 4px; }
.stat-value { font-size: 22px; font-weight: bold; }
.stat-value.request-count { color: #00d4ff; }
.stat-value.token-count { color: #fbbf24; }
.stat-detail { color: #555; font-size: 11px; margin-top: 2px; }
```

- [ ] **Step 7: Write `web/static/app.js`**

```javascript
function getCsrfToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.content : '';
}

function showAddUser() {
    document.getElementById('add-user-modal').style.display = 'flex';
    document.getElementById('new-user-name').focus();
}

function hideAddUser() {
    document.getElementById('add-user-modal').style.display = 'none';
    document.getElementById('new-user-name').value = '';
}

function hideAPIKey() {
    document.getElementById('api-key-modal').style.display = 'none';
    location.reload();
}

function copyAPIKey() {
    const key = document.getElementById('api-key-value').textContent;
    navigator.clipboard.writeText(key);
    document.querySelector('.btn-copy').textContent = 'Copied!';
}

async function createUser() {
    const name = document.getElementById('new-user-name').value.trim();
    if (!name) return;

    const formData = new URLSearchParams();
    formData.append('name', name);
    formData.append('csrf_token', getCsrfToken());

    const resp = await fetch('/admin/users', {
        method: 'POST',
        headers: { 'X-CSRF-Token': getCsrfToken() },
        body: formData,
    });

    if (!resp.ok) {
        alert('Failed to create user');
        return;
    }

    const data = await resp.json();
    hideAddUser();

    document.getElementById('api-key-value').textContent = data.api_key;
    document.getElementById('api-key-modal').style.display = 'flex';
}

async function deleteUser(id, name) {
    if (!confirm('Delete user "' + name + '"? This will also delete all their usage data.')) return;

    const resp = await fetch('/admin/users/' + id, {
        method: 'DELETE',
        headers: { 'X-CSRF-Token': getCsrfToken() },
    });

    if (resp.ok) {
        document.getElementById('user-row-' + id).remove();
    } else {
        alert('Failed to delete user');
    }
}

async function toggleActive(id, currentlyActive) {
    const resp = await fetch('/admin/users/' + id, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': getCsrfToken(),
        },
        body: JSON.stringify({ active: !currentlyActive }),
    });

    if (resp.ok) {
        location.reload();
    } else {
        alert('Failed to update user');
    }
}
```

- [ ] **Step 8: Verify it compiles**

```bash
go build ./web/...
```

- [ ] **Step 9: Commit**

```bash
git add web/
git commit -m "feat: add admin UI templates and static assets (dark theme)"
```

---

## Chunk 4: Usage Tracking — Stream Parser

### Task 8: Create stream usage tracker

**Files:**
- Create: `proxy/tracker.go`

- [ ] **Step 1: Write `proxy/tracker.go`**

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hieptran/copilot-proxy/db"
)

// UsageTracker collects usage data from proxy responses and writes to the database.
type UsageTracker struct {
	ch       chan db.UsageRecord
	database *db.DB
	wg       sync.WaitGroup
}

// NewUsageTracker creates a new tracker with a buffered channel and starts the writer goroutine.
func NewUsageTracker(database *db.DB) *UsageTracker {
	t := &UsageTracker{
		ch:       make(chan db.UsageRecord, 1000),
		database: database,
	}
	t.wg.Add(1)
	go t.writer()
	return t
}

// Track sends a usage record to the writer. Non-blocking; drops if channel is full.
func (t *UsageTracker) Track(record db.UsageRecord) {
	select {
	case t.ch <- record:
	default:
		log.Printf("WARNING: usage tracking channel full, dropping record for user %d", record.UserID)
	}
}

func (t *UsageTracker) writer() {
	defer t.wg.Done()
	var backoff time.Duration

	for record := range t.ch {
		err := t.database.InsertUsage(record)
		if err != nil {
			log.Printf("ERROR: failed to write usage log: %v", err)
			if backoff == 0 {
				backoff = 1 * time.Second
			} else {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			time.Sleep(backoff)
		} else {
			backoff = 0
		}
	}
}

// usageData represents the usage field in an OpenAI-style API response.
type usageData struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// streamResponse represents a chunk of an SSE stream data line.
type streamResponse struct {
	Usage *usageData `json:"usage,omitempty"`
	Model string     `json:"model,omitempty"`
}

// nonStreamResponse represents a non-streaming API response.
type nonStreamResponse struct {
	Usage *usageData `json:"usage,omitempty"`
	Model string     `json:"model,omitempty"`
}

// TrackingReader wraps a response body to intercept and extract usage data from the stream.
type TrackingReader struct {
	inner    io.ReadCloser
	lineBuf  string // buffer for incomplete lines across Read boundaries
	userID   int
	path     string
	tracker  *UsageTracker
	usage    *usageData
	model    string
}

// NewTrackingReader creates a reader that tees data through while scanning for usage info.
func NewTrackingReader(body io.ReadCloser, userID int, path string, tracker *UsageTracker) *TrackingReader {
	return &TrackingReader{
		inner:   body,
		userID:  userID,
		path:    path,
		tracker: tracker,
	}
}

// Read implements io.Reader. Passes data through and scans for usage info.
func (r *TrackingReader) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 {
		r.scanForUsage(p[:n])
	}
	if err == io.EOF {
		r.emitUsage()
	}
	return n, err
}

// Close implements io.Closer.
func (r *TrackingReader) Close() error {
	r.emitUsage()
	return r.inner.Close()
}

func (r *TrackingReader) scanForUsage(data []byte) {
	// Prepend any leftover data from previous Read call
	text := r.lineBuf + string(data)
	lines := strings.Split(text, "\n")

	// Last element may be incomplete — save for next Read
	r.lineBuf = lines[len(lines)-1]
	lines = lines[:len(lines)-1]

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			continue
		}

		var resp streamResponse
		if err := json.Unmarshal([]byte(payload), &resp); err != nil {
			continue
		}
		if resp.Model != "" {
			r.model = resp.Model
		}
		if resp.Usage != nil {
			r.usage = resp.Usage
		}
	}
}

func (r *TrackingReader) emitUsage() {
	if r.tracker == nil {
		return
	}
	tracker := r.tracker
	r.tracker = nil // emit only once

	record := db.UsageRecord{
		UserID: r.userID,
		Path:   r.path,
		Model:  r.model,
	}
	if r.usage != nil {
		record.PromptTokens = r.usage.PromptTokens
		record.CompletionTokens = r.usage.CompletionTokens
		record.TotalTokens = r.usage.TotalTokens
	}
	tracker.Track(record)
}

// ParseNonStreamUsage parses usage from a non-streaming JSON response body.
// Returns a new ReadCloser with the same content (re-serves the body).
func ParseNonStreamUsage(body io.ReadCloser, userID int, path string, tracker *UsageTracker) io.ReadCloser {
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		log.Printf("ERROR: failed to read response body for usage tracking: %v", err)
		return io.NopCloser(bytes.NewReader(data))
	}

	var resp nonStreamResponse
	if err := json.Unmarshal(data, &resp); err == nil && resp.Usage != nil {
		tracker.Track(db.UsageRecord{
			UserID:           userID,
			Path:             path,
			Model:            resp.Model,
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		})
	}

	return io.NopCloser(bytes.NewReader(data))
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./proxy/...
```

- [ ] **Step 3: Commit**

```bash
git add proxy/tracker.go
git commit -m "feat: add usage tracker with SSE stream parser and buffered writer"
```

---

## Chunk 5: Integration — Wire Everything Together

### Task 9: Update proxy server and main.go — wire everything together

**Files:**
- Modify: `proxy/server.go`
- Modify: `main.go`

**Note:** These two files must be updated together because `NewServer` signature changes from `(authenticator, port)` to `(authenticator, port, database)`. Updating them separately would break compilation.

- [ ] **Step 1: Update `proxy/server.go`**

Replace the entire file with the updated version that adds:
- API key validation middleware
- Usage tracking via `ModifyResponse`
- Admin route registration
- Static file serving

```go
package proxy

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/hieptran/copilot-proxy/admin"
	"github.com/hieptran/copilot-proxy/auth"
	"github.com/hieptran/copilot-proxy/config"
	"github.com/hieptran/copilot-proxy/db"
	"github.com/hieptran/copilot-proxy/web"
)

type contextKey string

const userContextKey contextKey = "user"

// Server is the reverse proxy server that forwards requests to GitHub Copilot.
type Server struct {
	authenticator *auth.Authenticator
	port          int
	proxy         *httputil.ReverseProxy
	database      *db.DB
	tracker       *UsageTracker
	adminHandler  *admin.Admin
	mux           *http.ServeMux
}

// NewServer creates a new proxy server.
func NewServer(authenticator *auth.Authenticator, port int, database *db.DB) (*Server, error) {
	target, err := url.Parse(config.CopilotAPIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Copilot API URL: %w", err)
	}

	s := &Server{
		authenticator: authenticator,
		port:          port,
		database:      database,
		mux:           http.NewServeMux(),
	}

	if database != nil {
		s.tracker = NewUsageTracker(database)
	}

	// Create reverse proxy
	s.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			if strings.HasPrefix(req.URL.Path, "/v1/") {
				req.URL.Path = "/" + strings.TrimPrefix(req.URL.Path, "/v1/")
			}

			token, err := s.authenticator.GetCopilotToken()
			if err != nil {
				log.Printf("ERROR: Failed to get Copilot token: %v", err)
				return
			}

			InjectHeaders(req, token)
			log.Printf("-> %s %s", req.Method, req.URL.Path)
		},
		ModifyResponse: func(resp *http.Response) error {
			log.Printf("<- %s %d %s", resp.Request.URL.Path, resp.StatusCode, resp.Status)

			if s.tracker == nil {
				return nil
			}

			// Extract user from request context
			user, ok := resp.Request.Context().Value(userContextKey).(*db.User)
			if !ok || user == nil {
				return nil
			}

			contentType := resp.Header.Get("Content-Type")
			path := resp.Request.URL.Path

			if strings.Contains(contentType, "text/event-stream") {
				// Streaming response — wrap body with tracking reader
				resp.Body = NewTrackingReader(resp.Body, user.ID, path, s.tracker)
			} else if strings.Contains(contentType, "application/json") {
				// Non-streaming response — parse body for usage
				resp.Body = ParseNonStreamUsage(resp.Body, user.ID, path, s.tracker)
			}

			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			log.Printf("Proxy error: %v", err)
			http.Error(w, fmt.Sprintf(`{"error": {"message": "proxy error: %s", "type": "proxy_error"}}`, err.Error()),
				http.StatusBadGateway)
		},
	}

	return s, nil
}

// SetupAdmin configures the admin UI routes.
func (s *Server) SetupAdmin(tmpl *template.Template, username, password string) error {
	s.adminHandler = admin.New(s.database, tmpl)
	if err := s.adminHandler.SetupAdmin(username, password); err != nil {
		return err
	}
	s.adminHandler.RegisterRoutes(s.mux)

	// Serve static files
	staticSub, err := fs.Sub(web.Static, "static")
	if err != nil {
		return err
	}
	s.mux.Handle("GET /admin/static/", http.StripPrefix("/admin/static/",
		http.FileServer(http.FS(staticSub))))

	return nil
}

// handleHealth returns the proxy health and auth status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	authenticated, expiry := s.authenticator.Status()
	status := "ok"
	if !authenticated {
		status = "unauthenticated"
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "%s", "authenticated": %t, "token_expiry": "%s"}`,
		status, authenticated, expiry.Format("2006-01-02T15:04:05Z07:00"))
}

// handleCORS handles CORS preflight requests.
func (s *Server) handleCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

// handleProxy handles API proxy requests with API key validation.
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	// CORS
	if r.Method == "OPTIONS" {
		s.handleCORS(w)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	// API key validation (only if database is configured)
	if s.database != nil {
		apiKey := extractAPIKey(r)
		if apiKey == "" {
			http.Error(w, `{"error": "missing api key"}`, http.StatusUnauthorized)
			return
		}

		user, err := s.database.GetUserByAPIKey(apiKey)
		if err != nil {
			log.Printf("ERROR: API key lookup failed: %v", err)
			http.Error(w, `{"error": "internal error"}`, http.StatusInternalServerError)
			return
		}
		if user == nil {
			http.Error(w, `{"error": "invalid api key"}`, http.StatusUnauthorized)
			return
		}
		if !user.Active {
			http.Error(w, `{"error": "user is inactive"}`, http.StatusForbidden)
			return
		}

		// Store user in context for usage tracking in ModifyResponse
		ctx := context.WithValue(r.Context(), userContextKey, user)
		r = r.WithContext(ctx)
	}

	s.proxy.ServeHTTP(w, r)
}

// Start starts the proxy server on the configured port.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)

	fmt.Println()
	fmt.Println("Copilot Proxy Server")
	fmt.Println("====================")
	fmt.Printf("Listening on: http://localhost%s\n", addr)
	fmt.Printf("Proxy target: %s\n", config.CopilotAPIBaseURL)
	fmt.Println()
	fmt.Printf("  Base URL: http://localhost:%d/v1\n", s.port)
	if s.database != nil {
		fmt.Println("  API Key:  (use key from admin panel)")
		fmt.Printf("  Admin UI: http://localhost:%d/admin/\n", s.port)
	} else {
		fmt.Println("  API Key:  any-value (proxy handles auth)")
	}
	fmt.Println()
	fmt.Println("All requests will use X-Initiator: agent (premium saver mode)")
	fmt.Println()

	// Register routes
	s.mux.HandleFunc("GET /health", s.handleHealth)
	// Catch-all: proxy everything not matched by admin routes
	s.mux.HandleFunc("/", s.handleProxy)

	return http.ListenAndServe(addr, s.mux)
}

func extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return auth
}
```

- [ ] **Step 2: Update `main.go`**

Replace the entire file (adds DB init, admin setup, template parsing):

```go
package main

import (
	"fmt"
	"html/template"
	"os"
	"strconv"
	"time"

	"github.com/hieptran/copilot-proxy/auth"
	"github.com/hieptran/copilot-proxy/config"
	"github.com/hieptran/copilot-proxy/db"
	"github.com/hieptran/copilot-proxy/proxy"
	"github.com/hieptran/copilot-proxy/web"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "login":
		cmdLogin()
	case "logout":
		cmdLogout()
	case "status":
		cmdStatus()
	case "serve":
		cmdServe()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Copilot Proxy - Save GitHub Copilot premium requests")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  copilot-proxy <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  login     Authenticate with GitHub Copilot (OAuth device flow)")
	fmt.Println("  logout    Clear stored authentication tokens")
	fmt.Println("  status    Show current authentication status")
	fmt.Println("  serve     Start the proxy server")
	fmt.Println("  help      Show this help message")
	fmt.Println()
	fmt.Println("Serve options:")
	fmt.Printf("  -p PORT   Port to listen on (default: %d)\n", config.DefaultPort)
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  ADMIN_USERNAME  Admin login username (default: admin)")
	fmt.Println("  ADMIN_PASSWORD  Admin login password (required for admin UI)")
	fmt.Println("  DB_PATH         SQLite database path (default: ~/.config/copilot-proxy/copilot-proxy.db)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  copilot-proxy login")
	fmt.Println("  copilot-proxy serve")
	fmt.Println("  ADMIN_PASSWORD=secret copilot-proxy serve -p 9090")
}

func cmdLogin() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	if err := authenticator.Login(); err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogout() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	if err := authenticator.Logout(); err != nil {
		fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully logged out. All tokens have been cleared.")
}

func cmdStatus() {
	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)
	authenticated, expiry := authenticator.Status()

	if !authenticated {
		fmt.Println("Status: Not authenticated")
		fmt.Println("Run 'copilot-proxy login' to authenticate.")
		return
	}

	fmt.Println("Status: Authenticated")
	if !expiry.IsZero() {
		remaining := time.Until(expiry)
		if remaining > 0 {
			fmt.Printf("Copilot token expires in: %s\n", remaining.Round(time.Second))
		} else {
			fmt.Println("Copilot token: expired (will auto-refresh on next request)")
		}
		fmt.Printf("Token expiry: %s\n", expiry.Format("2006-01-02 15:04:05"))
	}
}

func cmdServe() {
	port := config.DefaultPort

	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "-p" && i+1 < len(os.Args) {
			p, err := strconv.Atoi(os.Args[i+1])
			if err != nil || p < 1 || p > 65535 {
				fmt.Fprintf(os.Stderr, "Invalid port: %s\n", os.Args[i+1])
				os.Exit(1)
			}
			port = p
			i++
		}
	}

	store, err := auth.NewStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !store.HasOAuthToken() {
		fmt.Fprintf(os.Stderr, "Not authenticated. Run 'copilot-proxy login' first.\n")
		os.Exit(1)
	}

	authenticator := auth.NewAuthenticator(store)

	// Initialize database only if admin is configured
	var database *db.DB
	adminPassword := config.AdminPassword()
	if adminPassword != "" {
		dbPath, err := config.DBPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting DB path: %v\n", err)
			os.Exit(1)
		}
		database, err = db.New(dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer database.Close()
		database.StartCleanupJob()
	}

	server, err := proxy.NewServer(authenticator, port, database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	// Setup admin UI if password is configured
	if adminPassword != "" {
		tmpl, err := template.ParseFS(web.Templates, "templates/*.html")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing templates: %v\n", err)
			os.Exit(1)
		}
		if err := server.SetupAdmin(tmpl, config.AdminUsername(), adminPassword); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting up admin: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("WARNING: ADMIN_PASSWORD not set. Admin UI is disabled.")
		fmt.Println("Set ADMIN_PASSWORD environment variable to enable admin UI.")
		fmt.Println()
	}

	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./...
```

- [ ] **Step 3: Run a quick smoke test**

```bash
ADMIN_PASSWORD=test go run . serve &
sleep 2
# Health check
curl -s http://localhost:8787/health
# Admin login page should load
curl -s -o /dev/null -w "%{http_code}" http://localhost:8787/admin/login
# API without key should return 401
curl -s -o /dev/null -w "%{http_code}" http://localhost:8787/v1/models
# Kill server
kill %1
```

Expected:
- Health: returns JSON with status
- Admin login: 200
- API without key: 401

- [ ] **Step 4: Commit**

```bash
git add proxy/server.go main.go
git commit -m "feat: wire database, admin UI, API key validation, and usage tracking"
```

---

### Task 11: End-to-end manual test

- [ ] **Step 1: Start server with admin enabled**

```bash
ADMIN_PASSWORD=testpass go run . serve
```

- [ ] **Step 2: Test admin flow in browser**

Open `http://localhost:8787/admin/login`:
1. Login with `admin` / `testpass`
2. Click "Add User", enter a name
3. Copy the API key shown
4. Check the users table shows the new user
5. Click "report" — should show empty stats
6. Toggle active/inactive

- [ ] **Step 3: Test proxy with API key**

```bash
curl http://localhost:8787/v1/models \
  -H "Authorization: Bearer <api-key-from-step-2>"
```

Expected: should return models list from Copilot API (or auth error if Copilot token is expired).

- [ ] **Step 4: Test proxy without API key**

```bash
curl http://localhost:8787/v1/models
```

Expected: `401 {"error": "missing api key"}`

- [ ] **Step 5: Check report page shows tracked requests**

Go to the user's report page — should show the request from step 3.

- [ ] **Step 6: Commit any fixes discovered during testing**

---

## Chunk 6: Cleanup & Polish

### Task 12: Update .gitignore

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Add SQLite and superpowers to .gitignore**

```
*.db
*.db-wal
*.db-shm
.superpowers/
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add SQLite and .superpowers to gitignore"
```

---

### Task 13: Final verification

- [ ] **Step 1: Full build test**

```bash
go build -o copilot-proxy .
```

- [ ] **Step 2: Verify binary size**

```bash
ls -lh copilot-proxy
```

Expected: should be reasonable (likely 15-20MB with SQLite).

- [ ] **Step 3: Test backward compatibility — no ADMIN_PASSWORD**

```bash
./copilot-proxy serve
```

Expected: prints warning about ADMIN_PASSWORD not set, server still starts, proxy works without API key validation (legacy mode — database is not initialized when ADMIN_PASSWORD is not set, as implemented in Task 10).

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete user management and usage reporting feature"
```
