package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"localctl/internal/launchd"
	"localctl/internal/plistinfo"
)

// isHTMX reports whether the request was issued by htmx.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// ServiceRow is the view model for one row (and its detail panel).
type ServiceRow struct {
	HTMLID     string
	Label      string
	FileName   string
	PlistPath  string
	Program    string
	State      launchd.StateKind
	StateLabel string
	PID        string
	ExitCode   string
	Enabled    bool
	Loaded     bool
	ParseError string
	Agent      *plistinfo.Agent // nil when plist file is missing
	Runs       int
}

func htmlID(label string) string {
	sum := sha256.Sum256([]byte(label))
	return "svc-" + hex.EncodeToString(sum[:8])
}

// buildRows merges scanned plists with `launchctl list` output so that
// services loaded from outside ~/Library/LaunchAgents are also shown.
func buildRows() ([]*ServiceRow, error) {
	agents, err := plistinfo.ScanAgentsDir()
	if err != nil {
		return nil, err
	}
	rows := make([]*ServiceRow, 0, len(agents))

	// Services that have a plist file on disk.
	for _, a := range agents {
		svc, err := launchd.GetService(a.Label, a.Path)
		if err != nil {
			log.Printf("inspect %s: %v", a.Label, err)
			svc = &launchd.Service{Label: a.Label, PlistPath: a.Path, Enabled: true, State: launchd.StateIdle}
		}
		rows = append(rows, toRow(svc, a))
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].Label < rows[j].Label })
	return rows, nil
}

func toRow(svc *launchd.Service, agent *plistinfo.Agent) *ServiceRow {
	row := &ServiceRow{
		HTMLID:    htmlID(svc.Label),
		Label:     svc.Label,
		PlistPath: svc.PlistPath,
		Program:   svc.Program,
		State:     svc.State,
		Enabled:   svc.Enabled,
		Agent:     agent,
		Runs:      svc.Runs,
	}
	if agent != nil {
		row.FileName = agent.FileName
		row.ParseError = agent.ParseError
		if row.Program == "" && len(agent.ProgramArguments) > 0 {
			row.Program = agent.ProgramArguments[0]
		}
	}
	row.Loaded = hasRuntimeInfo(svc)
	if svc.PID != nil {
		row.PID = strconv.Itoa(*svc.PID)
	} else {
		row.PID = "-"
	}
	if svc.LastExitCode != nil {
		row.ExitCode = strconv.Itoa(*svc.LastExitCode)
	} else {
		row.ExitCode = "-"
	}
	switch svc.State {
	case launchd.StateRunning:
		row.StateLabel = "running"
	case launchd.StateExited:
		row.StateLabel = "exited"
	case launchd.StateFailed:
		row.StateLabel = fmt.Sprintf("exit %s", row.ExitCode)
	default:
		row.StateLabel = "not running"
	}
	return row
}

// hasRuntimeInfo reports whether launchctl knows about this service.
func hasRuntimeInfo(svc *launchd.Service) bool {
	return svc.Runs > 0 || svc.PID != nil || svc.LastExitCode != nil
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// handleIndex renders the main page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	rows, err := buildRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "index.html", rows)
}

// handleServicesPartial returns just the list of rows (used for 5s polling).
func (s *Server) handleServicesPartial(w http.ResponseWriter, r *http.Request) {
	rows, err := buildRows()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "services", rows)
}

// handleDetailPartial returns the expanded config detail for one service.
func (s *Server) handleDetailPartial(w http.ResponseWriter, r *http.Request) {
	label := r.URL.Query().Get("label")
	path := r.URL.Query().Get("path")
	if label == "" {
		http.Error(w, "missing label", http.StatusBadRequest)
		return
	}
	if path != "" && !strings.HasPrefix(path, "/") {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}

	row := &ServiceRow{HTMLID: htmlID(label), Label: label}
	if path != "" {
		agent := plistinfo.ParseAgent(path)
		row.Agent = agent
		row.FileName = agent.FileName
		row.ParseError = agent.ParseError
		row.PlistPath = path
	}
	if svc, err := launchd.GetService(label, path); err == nil {
		row.Program = svc.Program
		row.Enabled = svc.Enabled
		row.Loaded = hasRuntimeInfo(svc)
		row.Runs = svc.Runs
		row.PID = "-"
		row.ExitCode = "-"
		if svc.PID != nil {
			row.PID = strconv.Itoa(*svc.PID)
		}
		if svc.LastExitCode != nil {
			row.ExitCode = strconv.Itoa(*svc.LastExitCode)
		}
	} else {
		row.Enabled = true
		row.PID = "-"
		row.ExitCode = "-"
	}
	s.render(w, "service_detail", row)
}

// handleAction performs a management operation and returns the refreshed row.
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	label := r.URL.Query().Get("label")
	path := r.URL.Query().Get("path")
	op := r.URL.Query().Get("op")
	if label == "" || op == "" {
		http.Error(w, "missing label or op", http.StatusBadRequest)
		return
	}

	var err error
	switch op {
	case "start":
		err = launchd.Start(label)
	case "restart":
		err = launchd.Restart(label)
	case "stop":
		err = launchd.Stop(label)
	case "enable":
		err = launchd.Enable(label)
	case "disable":
		err = launchd.Disable(label)
	case "load":
		if path == "" {
			http.Error(w, "load requires path", http.StatusBadRequest)
			return
		}
		err = launchd.Load(path, label)
	case "unload":
		err = launchd.Unload(label)
	default:
		http.Error(w, "unknown op: "+op, http.StatusBadRequest)
		return
	}

	if err != nil {
		// Return the error inline in the row's action area.
		s.render(w, "service_row", &ServiceRow{
			HTMLID:     htmlID(label),
			Label:      label,
			FileName:   filepath.Base(path),
			PlistPath:  path,
			ParseError: err.Error(),
			Enabled:    true,
		})
		return
	}

	svc, gerr := launchd.GetService(label, path)
	if gerr != nil {
		http.Error(w, gerr.Error(), http.StatusInternalServerError)
		return
	}
	var agent *plistinfo.Agent
	if path != "" {
		agent = plistinfo.ParseAgent(path)
	}
	s.render(w, "service_row", toRow(svc, agent))
}

// handleLoginPage shows the login form.
func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login.html", nil)
}

// handleLoginSubmit checks the password and sets the session cookie.
func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if !s.cfg.CheckPassword(r.PostFormValue("password")) {
		s.render(w, "login.html", map[string]any{"Error": "密码错误"})
		return
	}
	setSessionCookie(w, s.cfg.SecretKey)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout clears the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
