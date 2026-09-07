package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"localctl/internal/launchd"
	"localctl/internal/plistinfo"
)

// Service is the JSON view of one service (list item and detail).
type Service struct {
	Label      string            `json:"label"`
	FileName   string            `json:"file_name,omitempty"`
	PlistPath  string            `json:"plist_path,omitempty"`
	Program    string            `json:"program,omitempty"`
	State      launchd.StateKind `json:"state"`
	PID        string            `json:"pid,omitempty"`
	ExitCode   string            `json:"exit_code,omitempty"`
	Enabled    bool              `json:"enabled"`
	Loaded     bool              `json:"loaded"`
	ParseError string            `json:"parse_error,omitempty"`
	Runs       int               `json:"runs,omitempty"`
	Agent      *plistinfo.Agent  `json:"agent,omitempty"`
}

func toService(svc *launchd.Service, agent *plistinfo.Agent) *Service {
	out := &Service{
		Label:     svc.Label,
		PlistPath: svc.PlistPath,
		Program:   svc.Program,
		State:     svc.State,
		Enabled:   svc.Enabled,
		Loaded:    svc.PID != nil || svc.LastExitCode != nil || svc.Runs > 0,
		Runs:      svc.Runs,
		Agent:     agent,
	}
	if agent != nil {
		out.FileName = agent.FileName
		out.ParseError = agent.ParseError
		if out.Program == "" && len(agent.ProgramArguments) > 0 {
			out.Program = agent.ProgramArguments[0]
		}
	}
	if svc.PID != nil {
		out.PID = strconv.Itoa(*svc.PID)
	}
	if svc.LastExitCode != nil {
		out.ExitCode = strconv.Itoa(*svc.LastExitCode)
	}
	if out.State == "" {
		out.State = launchd.StateIdle
	}
	return out
}

// buildServices merges scanned plists with launchctl runtime state.
func buildServices() ([]*Service, error) {
	agents, err := plistinfo.ScanAgentsDir()
	if err != nil {
		return nil, err
	}
	services := make([]*Service, 0, len(agents))
	for _, a := range agents {
		svc, err := launchd.GetService(a.Label, a.Path)
		if err != nil {
			log.Printf("inspect %s: %v", a.Label, err)
			svc = &launchd.Service{Label: a.Label, PlistPath: a.Path, Enabled: true, State: launchd.StateIdle}
		}
		services = append(services, toService(svc, a))
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Label < services[j].Label })
	return services, nil
}

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleHealth is an unauthenticated health probe.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().Format(time.RFC3339)})
}

// handleLogin issues a bearer token for a valid password.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求体必须是 JSON")
		return
	}
	if !s.cfg.CheckPassword(req.Password) {
		writeJSONError(w, http.StatusUnauthorized, "密码错误")
		return
	}
	token, expires := newToken(s.cfg.SecretKey)
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_at": expires.Format(time.RFC3339)})
}

// handleMe validates the current token.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleServices returns all services (polled by the frontend every 5s).
func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	services, err := buildServices()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"services": services})
}

// handleServiceDetail returns one service with full plist config.
func (s *Server) handleServiceDetail(w http.ResponseWriter, r *http.Request) {
	label := r.PathValue("label")
	path := r.URL.Query().Get("path")
	if label == "" {
		writeJSONError(w, http.StatusBadRequest, "missing label")
		return
	}

	var agent *plistinfo.Agent
	if path != "" {
		agent = plistinfo.ParseAgent(path)
	}
	svc, err := launchd.GetService(label, path)
	if err != nil {
		if agent == nil {
			writeJSONError(w, http.StatusNotFound, err.Error())
			return
		}
		// Unloaded service with a known plist: still return config info.
		svc = &launchd.Service{Label: label, PlistPath: path, Enabled: true, State: launchd.StateIdle}
		if disabled, derr := launchd.DisabledMap(); derr == nil {
			if isDisabled, ok := disabled[label]; ok {
				svc.Enabled = !isDisabled
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": toService(svc, agent)})
}

var validOps = map[string]bool{
	"start": true, "restart": true, "stop": true,
	"enable": true, "disable": true, "load": true, "unload": true,
}

// handleAction performs a management operation and returns the refreshed service.
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	label := r.PathValue("label")
	var req struct {
		Op   string `json:"op"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求体必须是 JSON")
		return
	}
	if !validOps[req.Op] {
		writeJSONError(w, http.StatusBadRequest, "未知操作: "+req.Op)
		return
	}
	if (req.Op == "load") != (req.Path != "") {
		writeJSONError(w, http.StatusBadRequest, "load 需要 path，其他操作不需要")
		return
	}

	var err error
	switch req.Op {
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
		err = launchd.Load(req.Path, label)
	case "unload":
		err = launchd.Unload(label)
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var agent *plistinfo.Agent
	if req.Path != "" {
		agent = plistinfo.ParseAgent(req.Path)
	}
	svc, err := launchd.GetService(label, req.Path)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": toService(svc, agent)})
}
