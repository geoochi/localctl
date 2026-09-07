package server

import (
	"log"
	"net/http"
	"os"

	"localctl/internal/config"
)

// Server is the localctl JSON API server.
type Server struct {
	cfg        *config.Config
	corsOrigin string
}

// New returns a ready Server.
func New(cfg *config.Config) *Server {
	return &Server{
		cfg:        cfg,
		corsOrigin: os.Getenv("LOCALCTL_CORS_ORIGIN"),
	}
}

// cors adds CORS headers for the configured origin (default Vite dev server).
func (s *Server) cors(next http.Handler) http.Handler {
	origin := s.corsOrigin
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin != "*" && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the HTTP mux with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("GET /api/services", s.requireAuth(s.handleServices))
	mux.HandleFunc("GET /api/services/{label}", s.requireAuth(s.handleServiceDetail))
	mux.HandleFunc("POST /api/services/{label}/actions", s.requireAuth(s.handleAction))

	return s.cors(mux)
}

// Serve runs the HTTP server (blocking).
func (s *Server) Serve(addr string) error {
	log.Printf("localctl API listening on http://%s", addr)
	return http.ListenAndServe(addr, s.Handler())
}
