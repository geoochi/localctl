package launchd

import (
	"fmt"
)

// StateKind is a normalized view of a service's runtime state.
type StateKind string

const (
	StateRunning StateKind = "running"
	StateExited  StateKind = "exited"
	StateFailed  StateKind = "failed"
	StateIdle    StateKind = "idle"
)

// Service is the aggregate view shown in the UI: runtime info + enabled flag.
type Service struct {
	Label        string    `json:"label"`
	PID          *int      `json:"pid"`
	State        StateKind `json:"state"`
	LastExitCode *int      `json:"last_exit_code"`
	Runs         int       `json:"runs"`
	Enabled      bool      `json:"enabled"`
	PlistPath    string    `json:"plist_path"`
	Program      string    `json:"program"`
}

// normalizeState maps launchctl print's "state" string to a StateKind.
func normalizeState(printState string, pid *int, lastExit *int) StateKind {
	switch {
	case pid != nil && *pid > 0:
		return StateRunning
	case printState == "running":
		return StateRunning
	case lastExit != nil && *lastExit != 0:
		return StateFailed
	case lastExit != nil:
		return StateExited
	default:
		return StateIdle
	}
}

// action runs a state-changing launchctl subcommand.
func action(args ...string) error {
	_, err := run(args...)
	return err
}

// Start starts a loaded service (kickstart).
func Start(label string) error { return action("kickstart", Domain()+"/"+label) }

// Restart force-restarts a loaded service (kickstart -k).
func Restart(label string) error { return action("kickstart", "-k", Domain()+"/"+label) }

// Stop sends SIGTERM to the service's running process.
func Stop(label string) error { return action("kill", "SIGTERM", Domain()+"/"+label) }

// Enable removes the service from the disabled set.
func Enable(label string) error { return action("enable", Domain()+"/"+label) }

// Disable adds the service to the persistent disabled set.
func Disable(label string) error { return action("disable", Domain()+"/"+label) }

// Unload boots the service out of the domain (stops + unregisters).
func Unload(label string) error { return action("bootout", Domain()+"/"+label) }

// Load bootstraps a plist into the domain (registers + runs if RunAtLoad).
func Load(plistPath, label string) error { return action("bootstrap", Domain(), plistPath) }

// GetService builds the aggregate Service view for one label.
// plistPath is only used as a fallback when the service is not loaded.
func GetService(label, plistPath string) (*Service, error) {
	svc := &Service{Label: label, PlistPath: plistPath, State: StateIdle, Enabled: true}

	if info, err := Print(label); err == nil {
		svc.PID = info.PID
		svc.LastExitCode = info.LastExitCode
		svc.Runs = info.Runs
		svc.State = normalizeState(info.State, info.PID, info.LastExitCode)
		svc.Enabled = true
		if svc.PlistPath == "" {
			svc.PlistPath = info.PlistPath
		}
		if info.Program != "" {
			svc.Program = info.Program
		} else if len(info.ProgramArguments) > 0 {
			svc.Program = info.ProgramArguments[0]
		}
	} else if disabled, derr := DisabledMap(); derr == nil {
		if isDisabled, ok := disabled[label]; ok {
			svc.Enabled = !isDisabled
		}
	} else {
		return nil, fmt.Errorf("inspect service %s: %w", label, err)
	}
	return svc, nil
}
