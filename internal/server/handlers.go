package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"localctl/internal/cron"
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

// handleHealth is a health probe.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().Format(time.RFC3339)})
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
	"delete": true,
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
	needsPath := req.Op == "load" || req.Op == "delete"
	if needsPath && req.Path == "" {
		writeJSONError(w, http.StatusBadRequest, req.Op+" 需要 path")
		return
	}
	if !needsPath && req.Path != "" {
		writeJSONError(w, http.StatusBadRequest, req.Op+" 不需要 path")
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
	case "delete":
		err = deleteAgent(label, req.Path)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
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

// deleteAgent boots the service out (if loaded) and removes its plist file.
// Only plists under ~/Library/LaunchAgents may be deleted.
func deleteAgent(label, path string) error {
	if path == "" {
		return fmt.Errorf("delete 需要 path")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents") + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(path)+string(filepath.Separator), dir) {
		return fmt.Errorf("只允许删除 ~/Library/LaunchAgents 下的 plist")
	}
	// bootout first; "not loaded" is fine.
	_ = launchd.Unload(label)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("删除 plist: %w", err)
	}
	return nil
}

// handleCron lists the user's crontab entries with import feasibility.
func (s *Server) handleCron(w http.ResponseWriter, r *http.Request) {
	entries, err := cron.Load()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// handleCronImport converts one crontab entry into a LaunchAgent:
// write plist → bootstrap → remove the original line from crontab.
func (s *Server) handleCronImport(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "无效的条目序号")
		return
	}
	entries, err := cron.Load()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var entry *cron.Entry
	for _, e := range entries {
		if e.Index == idx {
			entry = e
			break
		}
	}
	if entry == nil {
		writeJSONError(w, http.StatusNotFound, "条目不存在（crontab 可能已变化）")
		return
	}
	if entry.Imported {
		writeJSONError(w, http.StatusConflict, "该条目已导入过")
		return
	}

	cronPath := os.Getenv("PATH") // cron 默认 PATH 很短，继承当前 PATH 更接近用户预期
	if err := entry.Import(cronPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			writeJSONError(w, http.StatusConflict, "同名 plist 已存在")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := launchd.Bootstrap(cron.PlistPath(entry.Label)); err != nil {
		os.Remove(cron.PlistPath(entry.Label)) // roll back the file
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := cron.RemoveLine(entry); err != nil {
		// plist 已生效但 crontab 清理失败：如实报告，用户可手动处理
		writeJSONError(w, http.StatusMultiStatus, "已导入，但从 crontab 移除原条目失败: "+err.Error())
		return
	}

	svc, err := launchd.GetService(entry.Label, cron.PlistPath(entry.Label))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"service": toService(svc, plistinfo.ParseAgent(cron.PlistPath(entry.Label)))})
}
