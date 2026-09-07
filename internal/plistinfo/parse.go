// Package plistinfo scans ~/Library/LaunchAgents and parses plist config.
package plistinfo

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"howett.net/plist"
)

// Agent is a parsed LaunchAgent plist.
type Agent struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	FileName string `json:"file_name"`

	Program          string    `json:"program,omitempty"`
	ProgramArguments []string  `json:"program_arguments,omitempty"`
	RunAtLoad        bool      `json:"run_at_load,omitempty"`
	KeepAliveText    string    `json:"keep_alive_text,omitempty"`
	StartInterval    int       `json:"start_interval,omitempty"`
	StartCalendar    []string  `json:"start_calendar,omitempty"`
	WatchPaths       []string  `json:"watch_paths,omitempty"`
	QueueDirs        []string  `json:"queue_directories,omitempty"`
	StdOutPath       string    `json:"std_out_path,omitempty"`
	StdErrPath       string    `json:"std_err_path,omitempty"`
	WorkingDir       string    `json:"working_dir,omitempty"`
	UserName         string    `json:"user_name,omitempty"`
	GroupName        string    `json:"group_name,omitempty"`
	Environment      [][2]string `json:"environment,omitempty"`
	LowPriorityIO    bool      `json:"low_priority_io,omitempty"`
	ProcessType      string    `json:"process_type,omitempty"`
	// ParseError is set when the plist file could not be read/parsed.
	ParseError string `json:"parse_error,omitempty"`
}

// ScanAgentsDir returns all *.plist agents in ~/Library/LaunchAgents sorted by label.
func ScanAgentsDir() ([]*Agent, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, "Library", "LaunchAgents")

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var agents []*Agent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		agents = append(agents, ParseAgent(path))
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Label < agents[j].Label })
	return agents, nil
}

// ParseAgent reads and parses a single plist file.
func ParseAgent(path string) *Agent {
	agent := &Agent{
		Path:     path,
		FileName: filepath.Base(path),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		agent.ParseError = err.Error()
		return agent
	}

	var raw map[string]any
	if _, err := plist.Unmarshal(data, &raw); err != nil {
		agent.ParseError = err.Error()
		return agent
	}

	str := func(key string) string { s, _ := raw[key].(string); return s }
	boolean := func(key string) bool { b, _ := raw[key].(bool); return b }

	agent.Label = str("Label")
	agent.Program = str("Program")
	if args, ok := raw["ProgramArguments"].([]any); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				agent.ProgramArguments = append(agent.ProgramArguments, s)
			}
		}
	}
	if agent.Label == "" && len(agent.ProgramArguments) > 0 {
		agent.Label = agent.ProgramArguments[0]
	}
	agent.RunAtLoad = boolean("RunAtLoad")
	agent.KeepAliveText = describeKeepAlive(raw["KeepAlive"])
	if v, ok := raw["StartInterval"].(int); ok {
		agent.StartInterval = v
	}
	agent.StartCalendar = describeCalendar(raw["StartCalendarInterval"])
	if arr, ok := raw["WatchPaths"].([]any); ok {
		for _, p := range arr {
			if s, ok := p.(string); ok {
				agent.WatchPaths = append(agent.WatchPaths, s)
			}
		}
	}
	if arr, ok := raw["QueueDirectories"].([]any); ok {
		for _, p := range arr {
			if s, ok := p.(string); ok {
				agent.QueueDirs = append(agent.QueueDirs, s)
			}
		}
	}
	agent.StdOutPath = str("StandardOutPath")
	agent.StdErrPath = str("StandardErrorPath")
	agent.WorkingDir = str("WorkingDirectory")
	agent.UserName = str("UserName")
	agent.GroupName = str("GroupName")
	if env, ok := raw["EnvironmentVariables"].(map[string]any); ok {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if v, ok := env[k].(string); ok {
				agent.Environment = append(agent.Environment, [2]string{k, v})
			}
		}
	}
	agent.LowPriorityIO = boolean("LowPriorityIO")
	agent.ProcessType = str("ProcessType")
	return agent
}

// describeKeepAlive renders KeepAlive (bool or dict) as human-readable text.
func describeKeepAlive(v any) string {
	switch val := v.(type) {
	case bool:
		if val {
			return "always"
		}
		return ""
	case map[string]any:
		var parts []string
		for _, k := range []string{"SuccessfulExit", "Crashed", "NetworkState", "PathState", "OtherJobEnabled"} {
			if b, ok := val[k].(bool); ok {
				parts = append(parts, k+"="+fmtBool(b))
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

// describeCalendar renders StartCalendarInterval (dict or list of dicts).
func describeCalendar(v any) []string {
	describe := func(m map[string]any) string {
		var parts []string
		for _, k := range []string{"Minute", "Hour", "Day", "Weekday", "Month"} {
			if iv, ok := m[k].(int); ok {
				parts = append(parts, k+"="+strconv.Itoa(iv))
			}
		}
		return strings.Join(parts, " ")
	}
	switch val := v.(type) {
	case map[string]any:
		if s := describe(val); s != "" {
			return []string{s}
		}
	case []any:
		var out []string
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if s := describe(m); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	return nil
}

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
