package server

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"localctl/internal/config"
	"localctl/web"
)

// Server is the localctl web server.
type Server struct {
	cfg  *config.Config
	tmpl *template.Template
}

// New parses templates and returns a ready Server.
func New(cfg *config.Config) (*Server, error) {
	t, err := template.New("").Funcs(template.FuncMap{
		// string converts typed strings (e.g. launchd.StateKind) for eq comparisons.
		"string": func(v any) string { return fmt.Sprint(v) },
	}).ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{cfg: cfg, tmpl: t}, nil
}

// Handler builds the HTTP mux with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	staticRoot, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err) // embed layout is fixed at build time
	}
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))))

	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /login", s.handleLoginSubmit)
	mux.HandleFunc("GET /logout", s.handleLogout)

	mux.Handle("GET /{$}", s.requireAuth(s.handleIndex))
	mux.Handle("GET /partials/services", s.requireAuth(s.handleServicesPartial))
	mux.Handle("GET /partials/detail", s.requireAuth(s.handleDetailPartial))
	mux.Handle("POST /action", s.requireAuth(s.handleAction))

	return mux
}

// Serve runs the HTTP server (blocking).
func (s *Server) Serve(addr string) error {
	log.Printf("localctl listening on http://%s", addr)
	return http.ListenAndServe(addr, s.Handler())
}
