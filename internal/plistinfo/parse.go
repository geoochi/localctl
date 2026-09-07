// Package plistinfo scans ~/Library/LaunchAgents and parses plist config.
package plistinfo

import (
	"fmt"
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

	Program          string      `json:"program,omitempty"`
	ProgramArguments []string    `json:"program_arguments,omitempty"`
	RunAtLoad        bool        `json:"run_at_load,omitempty"`
	KeepAliveText    string      `json:"keep_alive_text,omitempty"`
	StartInterval    int         `json:"start_interval,omitempty"`
	StartCalendar    []string    `json:"start_calendar,omitempty"`
	WatchPaths       []string    `json:"watch_paths,omitempty"`
	QueueDirs        []string    `json:"queue_directories,omitempty"`
	StdOutPath       string      `json:"std_out_path,omitempty"`
	StdErrPath       string      `json:"std_err_path,omitempty"`
	WorkingDir       string      `json:"working_dir,omitempty"`
	UserName         string      `json:"user_name,omitempty"`
	GroupName        string      `json:"group_name,omitempty"`
	Environment      [][2]string `json:"environment,omitempty"`
	LowPriorityIO    bool        `json:"low_priority_io,omitempty"`
	ProcessType      string      `json:"process_type,omitempty"`
	// RunDescription is a human-readable summary of how the agent triggers.
	RunDescription string `json:"run_description,omitempty"`
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
	agent.StartInterval = toInt(raw["StartInterval"])
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
	agent.RunDescription = describeRun(agent)
	return agent
}

// describeRun builds a human-readable summary of how the agent is triggered.
func describeRun(a *Agent) string {
	var parts []string

	if a.RunAtLoad {
		p := "登录时启动"
		switch a.KeepAliveText {
		case "always":
			p += "，退出后自动重启"
		case "":
		default:
			p += "，" + a.KeepAliveText + " 时重启"
		}
		parts = append(parts, p)
	} else if a.KeepAliveText == "always" {
		parts = append(parts, "退出后自动重启")
	}

	if a.StartInterval > 0 {
		parts = append(parts, formatInterval(a.StartInterval))
	}
	if len(a.StartCalendar) > 0 {
		parts = append(parts, describeCalendarSummary(a.StartCalendar))
	}
	if len(a.WatchPaths) > 0 {
		parts = append(parts, "监视文件变化时启动")
	}
	if len(a.QueueDirs) > 0 {
		parts = append(parts, "队列目录有内容时启动")
	}
	if len(parts) == 0 {
		return "仅手动启动"
	}
	return strings.Join(parts, "；")
}

func formatInterval(sec int) string {
	switch {
	case sec < 60:
		return fmt.Sprintf("每 %d 秒运行一次", sec)
	case sec < 90*60:
		return fmt.Sprintf("每 %d 分钟运行一次", sec/60)
	default:
		return fmt.Sprintf("每 %g 小时运行一次", float64(sec)/3600)
	}
}

var weekdayNames = map[int]string{0: "周日", 1: "周一", 2: "周二", 3: "周三", 4: "周四", 5: "周五", 6: "周六"}

// describeCalendarSummary summarizes parsed StartCalendarInterval entries.
// Each entry string looks like "Hour=17 Minute=30 Weekday=1" (fields sorted).
func describeCalendarSummary(entries []string) string {
	type point struct{ hour, minute int }
	byTime := map[point][]int{} // time -> weekdays (empty = every day)
	hasDay := false

	parseField := func(s, key string) int {
		v := -1
		for _, part := range strings.Split(s, " ") {
			if k, val, ok := strings.Cut(part, "="); ok && k == key {
				if n, err := strconv.Atoi(val); err == nil {
					v = n
				}
			}
		}
		return v
	}

	for _, e := range entries {
		hour := parseField(e, "Hour")
		minute := parseField(e, "Minute")
		weekday := parseField(e, "Weekday")
		if parseField(e, "Day") >= 0 {
			hasDay = true
		}
		p := point{hour: hour, minute: minute}
		if weekday >= 0 {
			byTime[p] = append(byTime[p], weekday)
		} else {
			byTime[p] = append(byTime[p], -1) // no weekday restriction
		}
	}

	if hasDay {
		return fmt.Sprintf("每月特定日期运行，共 %d 个计划点", len(entries))
	}

	// group by weekday pattern
	byPattern := map[string][]point{} // pattern -> times
	for p, wds := range byTime {
		pattern := summarizeWeekdays(wds)
		byPattern[pattern] = append(byPattern[pattern], p)
	}

	var out []string
	for pattern, times := range byPattern {
		sort.Slice(times, func(i, j int) bool {
			if times[i].hour != times[j].hour {
				return times[i].hour < times[j].hour
			}
			return times[i].minute < times[j].minute
		})
		var timeStrs []string
		for _, t := range times {
			switch {
			case t.hour >= 0 && t.minute >= 0:
				timeStrs = append(timeStrs, fmt.Sprintf("%02d:%02d", t.hour, t.minute))
			case t.hour < 0 && t.minute >= 0:
				timeStrs = append(timeStrs, fmt.Sprintf("每小时第 %d 分", t.minute))
			case t.hour >= 0:
				timeStrs = append(timeStrs, fmt.Sprintf("%d 点", t.hour))
			}
		}
		if len(timeStrs) > 3 {
			timeStrs = append(timeStrs[:3], fmt.Sprintf("等 %d 个", len(times)))
		}
		out = append(out, pattern+" "+strings.Join(timeStrs, "、"))
	}
	sort.Strings(out)
	return strings.Join(out, "；")
}

// summarizeWeekdays classifies a weekday list (-1 = no restriction):
// every day / workdays / single day.
func summarizeWeekdays(wds []int) string {
	restricted := wds[:0]
	for _, w := range wds {
		if w >= 0 {
			restricted = append(restricted, w)
		}
	}
	if len(restricted) == 0 {
		return "每天"
	}
	wds = restricted
	set := map[int]bool{}
	for _, w := range wds {
		set[w] = true
	}
	workdays := true
	for _, w := range []int{1, 2, 3, 4, 5} {
		if !set[w] {
			workdays = false
		}
	}
	if workdays && len(set) == 5 {
		return "工作日"
	}
	if len(set) == 1 {
		for w := range set {
			return "每" + weekdayNames[w]
		}
	}
	var names []string
	for w := range set {
		names = append(names, weekdayNames[w])
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return "每" + strings.Join(names, "")
}

// toInt coerces plist integer values (decoded as int/uint64) to int.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	default:
		return 0
	}
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
			if iv := toInt(m[k]); iv != 0 || m[k] != nil {
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
