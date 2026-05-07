package connect_test

import (
	"os"
	"path/filepath"
	"testing"

	"go.moleculesai.app/cli/internal/connect"
)

func TestState_LoadMissingReturnsZero(t *testing.T) {
	dir := t.TempDir()
	got, err := connect.LoadState(dir, "ws-x")
	if err != nil {
		t.Fatalf("LoadState missing: %v", err)
	}
	if got.WorkspaceID != "ws-x" {
		t.Errorf("WorkspaceID: got %q, want ws-x", got.WorkspaceID)
	}
	if got.LastSinceID != "" {
		t.Errorf("LastSinceID: got %q, want empty", got.LastSinceID)
	}
}

func TestState_SaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	in := connect.State{WorkspaceID: "ws-1", LastSinceID: "act-42"}
	if err := connect.SaveState(dir, in); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := connect.LoadState(dir, "ws-1")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got != in {
		t.Errorf("roundtrip: got %+v, want %+v", got, in)
	}
}

func TestState_AtomicRenameProducesNoTmp(t *testing.T) {
	dir := t.TempDir()
	if err := connect.SaveState(dir, connect.State{WorkspaceID: "ws-1", LastSinceID: "x"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("found leftover tmp file: %s", e.Name())
		}
	}
}

func TestState_SaveRequiresWorkspaceID(t *testing.T) {
	if err := connect.SaveState(t.TempDir(), connect.State{}); err == nil {
		t.Error("expected error on empty WorkspaceID")
	}
}

func TestState_LoadCorruptedSurfaces(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(connect.StatePath(dir, "ws-broken"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := connect.LoadState(dir, "ws-broken")
	if err == nil {
		t.Error("expected error on corrupted file")
	}
}

func TestState_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	if err := connect.SaveState(dir, connect.State{WorkspaceID: "ws-perm", LastSinceID: "x"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(connect.StatePath(dir, "ws-perm"))
	if err != nil {
		t.Fatal(err)
	}
	// 0o600 — owner-only read/write. Tokens may end up here in future
	// state additions; lock it down from day 1.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm: got %o, want 600", perm)
	}
}
