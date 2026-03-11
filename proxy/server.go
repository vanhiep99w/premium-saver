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
	authenticator   *auth.Authenticator
	port            int
	proxy           *httputil.ReverseProxy
	database        *db.DB
	initiatorPolicy *InitiatorPolicy
	tracker         *UsageTracker
	adminHandler    *admin.Admin
	mux             *http.ServeMux
}

// NewServer creates a new proxy server.
func NewServer(authenticator *auth.Authenticator, port int, database *db.DB) (*Server, error) {
	target, err := url.Parse(config.CopilotAPIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Copilot API URL: %w", err)
	}

	s := &Server{
		authenticator:   authenticator,
		port:            port,
		database:        database,
		initiatorPolicy: NewInitiatorPolicy(config.InitiatorUserEvery()),
		mux:             http.NewServeMux(),
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

			// Inject stream_options to get usage data in streaming responses
			if req.Method == "POST" && req.Body != nil {
				injectStreamOptions(req)
			}

			token, err := s.authenticator.GetCopilotToken()
			if err != nil {
				log.Printf("ERROR: Failed to get Copilot token: %v", err)
				return
			}

			initiator := s.initiatorPolicy.NextInitiator()
			InjectHeaders(req, token, initiator)
			log.Print(formatOutgoingRequestLog(initiator, req.Method, req.URL.Path))
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
func (s *Server) SetupAdmin(tmpls map[string]*template.Template, username, password string) error {
	s.adminHandler = admin.New(s.database, tmpls, s.initiatorPolicy)
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
	fmt.Printf("X-Initiator policy: %d agent request(s), then 1 user request\n", s.initiatorPolicy.GetUserEvery())
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

func formatOutgoingRequestLog(initiator, method, path string) string {
	return fmt.Sprintf("-> %s %s %s", initiator, method, path)
}
