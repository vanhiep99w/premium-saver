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
	db                *db.DB
	sessions          *SessionManager
	tmpls             map[string]*template.Template
	throttle          *loginThrottle
	initiatorSettings InitiatorSettings
}

type InitiatorSettings interface {
	GetUserEvery() int
	SetUserEvery(int)
}

type loginThrottle struct {
	mu       sync.Mutex
	failures map[string]throttleState
}

type throttleState struct {
	count    int
	lastFail time.Time
}

// New creates a new Admin handler.
func New(database *db.DB, tmpls map[string]*template.Template, initiatorSettings InitiatorSettings) *Admin {
	return &Admin{
		db:                database,
		sessions:          NewSessionManager(),
		tmpls:             tmpls,
		throttle:          &loginThrottle{failures: make(map[string]throttleState)},
		initiatorSettings: initiatorSettings,
	}
}

// SetupAdmin seeds the admin account from env vars if needed.
// On first run, creates the admin row. On subsequent runs, only updates if password changed.
func (a *Admin) SetupAdmin(username, password string) error {
	var storedHash string
	var storedUsername string
	err := a.db.Conn().QueryRow("SELECT username, password_hash FROM admin WHERE id = 1").Scan(&storedUsername, &storedHash)
	if err == nil {
		// Row exists — only update if username changed or password differs
		if storedUsername == username && bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil {
			return nil // no changes needed
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

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
	mux.HandleFunc("GET /admin/settings", a.requireAuth(a.handleSettingsPage))
	mux.HandleFunc("POST /admin/settings", a.requireAuth(a.handleSettingsUpdate))
	mux.HandleFunc("POST /admin/users", a.requireAuth(a.handleCreateUser))
	mux.HandleFunc("DELETE /admin/users/{id}", a.requireAuth(a.handleDeleteUser))
	mux.HandleFunc("PATCH /admin/users/{id}", a.requireAuth(a.handleUpdateUser))
	mux.HandleFunc("GET /admin/report/{id}", a.requireAuth(a.handleReportPage))
	mux.HandleFunc("GET /admin/api/report/{id}", a.requireAuth(a.handleReportAPI))
	mux.HandleFunc("GET /admin/api/report/{id}/chart-data", a.requireAuth(a.handleChartDataAPI))
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
	a.tmpls["login"].Execute(w, map[string]any{
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

	type userWithStats struct {
		db.User
		Requests24h    int
		TotalTokensStr string
	}
	var usersWithStats []userWithStats
	for _, u := range users {
		count, _ := a.db.GetRequestCount24h(u.ID)
		tokens, _ := a.db.GetTotalTokens(u.ID)
		usersWithStats = append(usersWithStats, userWithStats{
			User:           u,
			Requests24h:    count,
			TotalTokensStr: formatTokens(tokens),
		})
	}

	csrf := r.Header.Get("X-CSRF-Token")
	a.tmpls["users"].ExecuteTemplate(w, "layout", map[string]any{
		"Users":     usersWithStats,
		"CSRFToken": csrf,
		"ActiveNav": "users",
	})
}

func (a *Admin) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	csrf := r.Header.Get("X-CSRF-Token")
	a.tmpls["settings"].ExecuteTemplate(w, "layout", map[string]any{
		"CSRFToken": csrf,
		"UserEvery": a.initiatorSettings.GetUserEvery(),
		"ActiveNav": "settings",
	})
}

func (a *Admin) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	userEvery, err := strconv.Atoi(r.FormValue("user_every"))
	if err != nil || userEvery < 1 {
		http.Error(w, `{"error": "user_every must be a positive integer"}`, http.StatusBadRequest)
		return
	}

	a.initiatorSettings.SetUserEvery(userEvery)
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

func (a *Admin) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	if name == "" {
		http.Error(w, `{"error": "name is required"}`, http.StatusBadRequest)
		return
	}
	if len(name) > 255 {
		http.Error(w, `{"error": "name too long (max 255 characters)"}`, http.StatusBadRequest)
		return
	}

	user, apiKey, err := a.db.CreateUser(name)
	if err != nil {
		writeJSONError(w, err.Error(), http.StatusInternalServerError)
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
		writeJSONError(w, err.Error(), http.StatusNotFound)
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
		writeJSONError(w, err.Error(), http.StatusNotFound)
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
	a.tmpls["report"].ExecuteTemplate(w, "layout", map[string]any{
		"User":           user,
		"Stats":          stats,
		"RecentRequests": recentReqs,
		"CSRFToken":      csrf,
		"ActiveNav":      "users",
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

func (a *Admin) handleChartDataAPI(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error": "invalid id"}`, http.StatusBadRequest)
		return
	}

	hourly, err := a.db.GetHourlyUsage(id, 24)
	if err != nil {
		http.Error(w, `{"error": "internal error"}`, http.StatusInternalServerError)
		return
	}

	models, err := a.db.GetModelBreakdown(id, 30)
	if err != nil {
		http.Error(w, `{"error": "internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"hourly_usage":    hourly,
		"model_breakdown": models,
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

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// formatCost formats a cost value into a compact string.
func formatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	if cost < 1 {
		return fmt.Sprintf("$%.3f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

// Examples: 0, 542, 1.2k, 25k, 100k, 1.5M, 12M
func formatTokens(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1_000_000 {
		k := float64(n) / 1000
		if k < 10 {
			return fmt.Sprintf("%.1fk", k)
		}
		return fmt.Sprintf("%.0fk", k)
	}
	m := float64(n) / 1_000_000
	if m < 10 {
		return fmt.Sprintf("%.1fM", m)
	}
	return fmt.Sprintf("%.0fM", m)
}
