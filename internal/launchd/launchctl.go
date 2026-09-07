package launchd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const launchctlTimeout = 10 * time.Second

// run executes a launchctl command and returns stdout.
func run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), launchctlTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/launchctl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("launchctl %s: timed out", strings.Join(args, " "))
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("launchctl %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// Domain is the user gui domain, e.g. "gui/501".
func Domain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

// PrintInfo is the runtime state extracted from `launchctl print` text output.
type PrintInfo struct {
	Label            string
	PID              *int
	State            string // "running" / "not running"
	LastExitCode     *int   // nil when "(never exited)"
	Runs             int
	PlistPath        string
	Program          string
	ProgramArguments []string
}

// parseBlock parses a "{ key = value ... };" block from print output.
// entries are lines with the same tab depth as the block opener.
func parseBlock(lines []string, idx *int, depth int) map[string]any {
	m := map[string]any{}
	for *idx < len(lines) {
		raw := lines[*idx]
		trimmed := strings.TrimSpace(raw)
		*idx++
		if trimmed == "};" || trimmed == "}" {
			return m
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") && !strings.Contains(trimmed, "=>") {
			continue // bare quoted block member (e.g. inside program arguments)
		}

		// "key" => value  (maps like disabled services / environment)
		if key, val, ok := strings.Cut(trimmed, "=>"); ok {
			k := strings.Trim(strings.TrimSpace(key), "\"")
			m[k] = strings.Trim(strings.TrimSpace(val), "\"")
			continue
		}

		key, val, ok := strings.Cut(trimmed, " = ")
		if !ok {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), "\"")
		val = strings.TrimSpace(val)
		switch val {
		case "{":
			m[key] = parseBlock(lines, idx, depth+1)
		case "{ ... }":
			m[key] = nil // collapsed block
		default:
			m[key] = strings.Trim(val, "\"")
		}
	}
	return m
}

var disabledRe = regexp.MustCompile(`"([^"]+)" => (disabled|enabled)`)

// Print fetches runtime info for a single service by parsing text output.
func Print(label string) (*PrintInfo, error) {
	out, err := run("print", Domain()+"/"+label)
	if err != nil {
		return nil, err
	}

	// The output is one "<target> = { ... }" block; parse it into root.
	lines := strings.Split(out, "\n")
	root := map[string]any{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if _, val, ok := strings.Cut(trimmed, " = "); ok && val == "{" {
			idx := i + 1
			root = parseBlock(lines, &idx, 1)
			break
		}
	}

	info := &PrintInfo{
		State:     asString(root["state"]),
		Runs:      asInt(root["runs"]),
		PlistPath: asString(root["path"]),
	}
	if s := asString(root["label"]); s != "" {
		info.Label = s
	}
	if s := asString(root["program"]); s != "" {
		info.Program = s
	}
	if s := asString(root["pid"]); s != "" {
		if pid, err := strconv.Atoi(s); err == nil {
			info.PID = &pid
		}
	}
	if s := asString(root["last exit code"]); s != "" && s != "(never exited)" {
		if code, err := strconv.Atoi(s); err == nil {
			info.LastExitCode = &code
		}
	}
	for _, arg := range strings.Split(blockText(out, "arguments"), "\n") {
		arg = strings.Trim(strings.TrimSpace(arg), "\"")
		if arg != "" && arg != "}" {
			info.ProgramArguments = append(info.ProgramArguments, arg)
		}
	}
	return info, nil
}

// blockText extracts the raw text of a named "{...}" block from print output.
func blockText(out, key string) string {
	marker := "\t" + key + " = {"
	start := strings.Index(out, marker)
	if start < 0 {
		return ""
	}
	rest := out[start+len(marker):]
	end := strings.Index(rest, "\n\t}")
	if end < 0 {
		end = strings.Index(rest, "}")
	}
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any) int {
	if s, ok := v.(string); ok {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return 0
}

// DisabledMap returns label -> disabled from `launchctl print-disabled`.
func DisabledMap() (map[string]bool, error) {
	out, err := run("print-disabled", Domain())
	if err != nil {
		return nil, err
	}
	m := map[string]bool{}
	for _, match := range disabledRe.FindAllStringSubmatch(out, -1) {
		m[match[1]] = match[2] == "disabled"
	}
	return m, nil
}
