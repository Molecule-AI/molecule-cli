package connect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// State is the persisted per-workspace state for crash-resume. Currently
// just the activity cursor; future keys (auth token rotation, last
// successful heartbeat) get appended without a schema bump because
// json.Decode tolerates unknown / missing fields.
type State struct {
	WorkspaceID string `json:"workspace_id"`
	LastSinceID string `json:"last_since_id,omitempty"`
}

// StatePath returns the on-disk path for workspaceID's state file.
// dir is the directory root (typically `~/.config/molecule/state`); the
// caller resolves it via DefaultStateDir() unless the user passed a
// custom one.
func StatePath(dir, workspaceID string) string {
	return filepath.Join(dir, workspaceID+".json")
}

// DefaultStateDir returns ~/.config/molecule/state, creating the
// hierarchy if missing. Returns the path even on mkdir error so callers
// can surface a meaningful "could not write state" message — the loops
// run regardless; persistence is best-effort.
func DefaultStateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "molecule", "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return dir, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// LoadState reads workspaceID's state from dir. Returns a zero-value
// State (no error) when the file is missing — first run is the same
// as "fresh state". Other errors (parse failure, permission denied)
// surface so the user knows their state is corrupt.
func LoadState(dir, workspaceID string) (State, error) {
	path := StatePath(dir, workspaceID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{WorkspaceID: workspaceID}, nil
		}
		return State{}, fmt.Errorf("read %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.WorkspaceID == "" {
		s.WorkspaceID = workspaceID
	}
	return s, nil
}

// SaveState writes workspaceID's state atomically (write to .tmp, rename
// over). The rename is atomic on POSIX so a crash mid-write can never
// produce a half-written cursor file.
func SaveState(dir string, s State) error {
	if s.WorkspaceID == "" {
		return fmt.Errorf("save state: workspace_id is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := StatePath(dir, s.WorkspaceID)
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
