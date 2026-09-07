package server

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"localctl/frontend"
)

// Server is the localctl HTTP server: JSON API + embedded frontend.
// Listens on localhost only, no authentication.
type Server struct {
	corsOrigin string
}

// New returns a ready Server.
func New() *Server {
	return &Server{corsOrigin: os.Getenv("LOCALCTL_CORS_ORIGIN")}
}

// cors adds CORS headers only when LOCALCTL_CORS_ORIGIN is explicitly set
// (same-origin embedded frontend needs none; external API clients may opt in).
func (s *Server) cors(next http.Handler) http.Handler {
	origin := s.corsOrigin
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Handler builds the HTTP mux: /api routes plus the embedded SPA.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/services", s.handleServices)
	mux.HandleFunc("GET /api/services/{label}", s.handleServiceDetail)
	mux.HandleFunc("GET /api/services/{label}/source", s.handleSource)
	mux.HandleFunc("POST /api/services/{label}/actions", s.handleAction)
	mux.HandleFunc("POST /api/plist", s.handleCreatePlist)
	mux.HandleFunc("GET /api/cron", s.handleCron)
	mux.HandleFunc("POST /api/cron/{index}/import", s.handleCronImport)

	mux.Handle("/", s.frontendHandler())

	return s.cors(mux)
}

// frontendHandler serves the embedded Vite dist with SPA fallback to index.html.
func (s *Server) frontendHandler() http.Handler {
	distRoot, err := fs.Sub(frontend.Dist, "dist")
	if err != nil {
		panic(err) // embed layout is fixed at build time
	}
	fileServer := http.FileServer(http.FS(distRoot))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(distRoot, path); err != nil {
			r.URL.Path = "/" // unknown path → SPA entry
		}
		fileServer.ServeHTTP(w, r)
	})
}

// Serve runs the HTTP server (blocking).
func (s *Server) Serve(addr string) error {
	log.Printf("localctl listening on http://%s", addr)
	return http.ListenAndServe(addr, s.Handler())
}
