package proxy

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/hieptran/copilot-proxy/auth"
	"github.com/hieptran/copilot-proxy/config"
)

// Server is the reverse proxy server that forwards requests to GitHub Copilot.
type Server struct {
	authenticator *auth.Authenticator
	port          int
	proxy         *httputil.ReverseProxy
}

// NewServer creates a new proxy server.
func NewServer(authenticator *auth.Authenticator, port int) (*Server, error) {
	target, err := url.Parse(config.CopilotAPIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Copilot API URL: %w", err)
	}

	s := &Server{
		authenticator: authenticator,
		port:          port,
	}

	// Create reverse proxy
	s.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			// Set the target host
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Strip /v1 prefix - Copilot API uses /chat/completions, not /v1/chat/completions
			if strings.HasPrefix(req.URL.Path, "/v1/") {
				req.URL.Path = "/" + strings.TrimPrefix(req.URL.Path, "/v1/")
			}

			// Get a valid Copilot token
			token, err := s.authenticator.GetCopilotToken()
			if err != nil {
				log.Printf("ERROR: Failed to get Copilot token: %v", err)
				return
			}

			// Inject required headers
			InjectHeaders(req, token)

			log.Printf("-> %s %s", req.Method, req.URL.Path)
		},
		ModifyResponse: func(resp *http.Response) error {
			log.Printf("<- %s %d %s", resp.Request.URL.Path, resp.StatusCode, resp.Status)
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

// ServeHTTP handles incoming HTTP requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle health check
	if r.URL.Path == "/health" {
		s.handleHealth(w, r)
		return
	}

	// Handle CORS preflight
	if r.Method == "OPTIONS" {
		s.handleCORS(w)
		return
	}

	// Set CORS headers on all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	// Forward all API requests to Copilot (strip /v1 prefix in Director)
	// Supports: /chat/completions, /responses, /models, /embeddings, and any future endpoints
	s.proxy.ServeHTTP(w, r)
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

// Start starts the proxy server on the configured port.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)

	fmt.Println()
	fmt.Println("Copilot Proxy Server")
	fmt.Println("====================")
	fmt.Printf("Listening on: http://localhost%s\n", addr)
	fmt.Printf("Proxy target: %s\n", config.CopilotAPIBaseURL)
	fmt.Println()
	fmt.Println("Configure your AI client with:")
	fmt.Printf("  Base URL: http://localhost:%d/v1\n", s.port)
	fmt.Println("  API Key:  any-value (proxy handles auth)")
	fmt.Println()
	fmt.Println("All requests will use X-Initiator: agent (premium saver mode)")
	fmt.Println()

	mux := http.NewServeMux()
	mux.Handle("/", s)

	return http.ListenAndServe(addr, mux)
}
