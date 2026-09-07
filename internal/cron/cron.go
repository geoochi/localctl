// Package cron reads the user's crontab and converts entries to launchd
// LaunchAgents where the schedule is representable.
//
// Representable:
//   - @reboot (@hourly/@daily/... expand to StartCalendarInterval)
//   - "m h * * *" style entries whose fields are plain numbers or comma lists
//   - "*/n * * * *" (minute step) → StartInterval approximation (n*60 seconds,
//     launchd cannot align to wall clock, so it may drift)
//
// Ranges (10-22), steps (*/5, 0-30/2) and comma lists are expanded into
// explicit StartCalendarInterval dicts. Not representable: month/weekday names
// (JAN/MON) and restricted day-of-month AND day-of-week in one entry (launchd
// combines them with AND while cron uses OR).
package cron

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"howett.net/plist"
)

// Entry is one parsed crontab job line.
type Entry struct {
	Index       int    `json:"index"`                 // line index in the crontab
	Raw         string `json:"raw"`                   // original line
	Schedule    string `json:"schedule"`              // schedule expression (expanded for @shortcuts)
	Command     string `json:"command"`
	Importable  bool   `json:"importable"`
	Approximate bool   `json:"approximate,omitempty"` // StartInterval approximation
	Reason      string `json:"reason,omitempty"`      // explanation for non-importable / approximate entries
	Label       string `json:"label"`
	Imported    bool   `json:"imported"` // matching plist already exists in ~/Library/LaunchAgents

	// parsed launchd representation (not serialized)
	cal        []calendarDict
	intervalSec int
	runAtLoad  bool
}

// calendarDict is one StartCalendarInterval dict.
type calendarDict map[string]int

const (
	agentDirName  = "Library/LaunchAgents"
	labelPrefix   = "com.localctl.cron."
	maxExpandDict = 200
)

func launchAgentsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, agentDirName)
}

// PlistPath returns the plist path for a generated cron agent label.
func PlistPath(label string) string {
	return filepath.Join(launchAgentsDir(), label+".plist")
}

// Load reads and parses the user crontab. Empty crontab → empty list, no error.
func Load() ([]*Entry, error) {
	cmd := exec.Command("/usr/bin/crontab", "-l")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(strings.ToLower(stderr.String()), "no crontab for") {
			return []*Entry{}, nil
		}
		return nil, fmt.Errorf("read crontab: %s", strings.TrimSpace(stderr.String()))
	}
	return Parse(string(out)), nil
}

var envLineRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)

// Parse converts crontab text into entries (job lines only).
func Parse(text string) []*Entry {
	var entries []*Entry
	for idx, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || envLineRe.MatchString(trimmed) {
			continue
		}

		var schedFields []string
		var cmd string
		if strings.HasPrefix(trimmed, "@") {
			m := strings.IndexAny(trimmed, " \t")
			if m < 0 {
				continue
			}
			schedFields = []string{trimmed[:m]}
			cmd = strings.TrimSpace(trimmed[m+1:])
		} else {
			fields := strings.Fields(trimmed)
			if len(fields) < 6 {
				continue
			}
			schedFields = fields[:5]
			cmd = strings.Join(fields[5:], " ")
		}
		if cmd == "" {
			continue
		}

		e := &Entry{
			Index:    idx,
			Raw:      trimmed,
			Schedule: strings.Join(schedFields, " "),
			Command:  cmd,
			Label:    labelFor(trimmed),
		}
		cal, interval, approx, reason := parseSchedule(schedFields)
		if reason != "" && !approx {
			e.Importable = false
			e.Reason = reason
		} else {
			e.Importable = true
			e.Approximate = approx
			e.Reason = reason
			e.cal = cal
			e.intervalSec = interval
			e.runAtLoad = schedFields[0] == "@reboot"
		}
		e.Imported = plistExists(e.Label)
		entries = append(entries, e)
	}
	return entries
}

func plistExists(label string) bool {
	_, err := os.Stat(PlistPath(label))
	return err == nil
}

func labelFor(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return labelPrefix + hex.EncodeToString(sum[:4])
}

var numRe = regexp.MustCompile(`^\d{1,2}$`)

// parseSchedule converts a cron schedule into launchd structures.
func parseSchedule(fields []string) (cal []calendarDict, intervalSec int, approx bool, reason string) {
	switch fields[0] {
	case "@reboot":
		return nil, 0, false, "" // RunAtLoad, no schedule dicts needed
	case "@hourly":
		fields = []string{"0", "*", "*", "*", "*"}
	case "@daily", "@midnight":
		fields = []string{"0", "0", "*", "*", "*"}
	case "@weekly":
		fields = []string{"0", "0", "*", "*", "0"}
	case "@monthly":
		fields = []string{"0", "0", "1", "*", "*"}
	case "@yearly", "@annually":
		fields = []string{"0", "0", "1", "1", "*"}
	}
	if len(fields) != 5 {
		return nil, 0, false, "无法识别的调度表达式"
	}

	// "*/n * * * *" → StartInterval approximation (avoids huge dict expansion)
	if strings.HasPrefix(fields[0], "*/") {
		if n, err := strconv.Atoi(strings.TrimPrefix(fields[0], "*/")); err == nil && n > 0 && allStars(fields[1:]) {
			return nil, n * 60, true, fmt.Sprintf("launchd 无法对齐整点，按每 %d 秒近似（会缓慢漂移）", n*60)
		}
	}

	var sets [5][]int
	names := [5]string{"Minute", "Hour", "Day", "Month", "Weekday"}
	bounds := [5][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for i, f := range fields {
		if f == "*" {
			continue
		}
		vals, err := expandField(f, bounds[i][0], bounds[i][1])
		if err != "" {
			return nil, 0, false, err
		}
		sets[i] = vals
	}

	// cron treats restricted DOM and DOW as OR; launchd ANDs them.
	if len(sets[2]) > 0 && len(sets[4]) > 0 {
		return nil, 0, false, "同时限制“日”与“星期”时 cron 是或语义、launchd 是且语义，无法等价"
	}

	total := 1
	for i := range sets {
		if len(sets[i]) > 0 {
			total *= len(sets[i])
		}
	}
	if total > maxExpandDict {
		return nil, 0, false, "展开后的时间点过多"
	}

	cal = make([]calendarDict, 0, total)
	for i := 0; i < total; i++ {
		d := calendarDict{}
		n := i
		for j := 4; j >= 0; j-- {
			if len(sets[j]) > 0 {
				d[names[j]] = sets[j][n%len(sets[j])]
				n /= len(sets[j])
			}
		}
		cal = append(cal, d)
	}
	return cal, 0, false, ""
}

// expandField expands one cron field into explicit values.
// Supported: number, a-b, a-b/n, */n, comma lists of these.
func expandField(f string, min, max int) ([]int, string) {
	var out []int
	for _, part := range strings.Split(f, ",") {
		var lo, hi, step int
		switch {
		case part == "*" || strings.HasPrefix(part, "*/"):
			if part == "*" {
				lo, hi, step = min, max, 1
			} else {
				n, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
				if err != nil || n <= 0 {
					return nil, fmt.Sprintf("字段 %q 步进无效", f)
				}
				lo, hi, step = min, max, n
			}
		default:
			rangePart, stepPart, hasStep := strings.Cut(part, "/")
			step = 1
			if hasStep {
				n, err := strconv.Atoi(stepPart)
				if err != nil || n <= 0 {
					return nil, fmt.Sprintf("字段 %q 步进无效", f)
				}
				step = n
			}
			a, b, isRange := strings.Cut(rangePart, "-")
			v, err := strconv.Atoi(a)
			if err != nil {
				return nil, fmt.Sprintf("字段 %q 含名称或非法数字（launchd 不支持 MON/JAN 等写法）", f)
			}
			lo = v
			hi = v
			if isRange {
				v, err = strconv.Atoi(b)
				if err != nil {
					return nil, fmt.Sprintf("字段 %q 区间无效", f)
				}
				hi = v
			}
			if lo > hi {
				return nil, fmt.Sprintf("字段 %q 区间倒置", f)
			}
		}
		if lo < min || hi > max {
			return nil, fmt.Sprintf("字段 %q 超出范围", f)
		}
		for v := lo; v <= hi; v += step {
			out = append(out, v)
		}
	}
	return out, ""
}

func allStars(fields []string) bool {
	for _, f := range fields {
		if f != "*" {
			return false
		}
	}
	return true
}

// Import writes the LaunchAgent plist for an importable entry.
// Returns os.ErrExist if a plist with the same label is already present.
func (e *Entry) Import(envPath string) error {
	if !e.Importable {
		return fmt.Errorf("该条目无法导入: %s", e.Reason)
	}
	if plistExists(e.Label) {
		return os.ErrExist
	}

	dict := map[string]any{
		"Label":            e.Label,
		"ProgramArguments": []string{"/bin/sh", "-c", e.Command},
	}
	if e.runAtLoad {
		dict["RunAtLoad"] = true
	}
	if e.intervalSec > 0 {
		dict["StartInterval"] = int64(e.intervalSec)
	}
	if len(e.cal) > 0 {
		if len(e.cal) == 1 {
			dict["StartCalendarInterval"] = e.cal[0]
		} else {
			dict["StartCalendarInterval"] = e.cal
		}
	}
	if envPath != "" {
		dict["EnvironmentVariables"] = map[string]any{"PATH": envPath}
	}

	data, err := plist.MarshalIndent(dict, plist.XMLFormat, "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(PlistPath(e.Label), data, 0o644)
}

// RemoveLine removes the entry's original line from the user crontab.
func RemoveLine(e *Entry) error {
	out, err := exec.Command("/usr/bin/crontab", "-l").Output()
	if err != nil {
		return fmt.Errorf("read crontab: %w", err)
	}
	var kept []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == e.Raw {
			continue
		}
		kept = append(kept, line)
	}
	content := strings.Join(kept, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	cmd := exec.Command("/usr/bin/crontab", "-")
	cmd.Stdin = strings.NewReader(content)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("write crontab: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
