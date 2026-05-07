package claudecode_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"go.moleculesai.app/cli/internal/backends"
	_ "go.moleculesai.app/cli/internal/backends/claudecode" // register
)

func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only: shell command semantics differ on Windows")
	}
}

func TestClaudeCode_Registered(t *testing.T) {
	names := backends.Names()
	found := false
	for _, n := range names {
		if n == "claude-code" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("claude-code missing from registry: %v", names)
	}
}

// TestClaudeCode_BinArgsTranslation: the wrapper should compose bin +
// "-p" + extra-args into the underlying exec command. Use bin=echo
// args=hello so the command becomes "echo -p hello" and stdout is
// deterministic.
func TestClaudeCode_BinArgsTranslation(t *testing.T) {
	requireUnix(t)
	be, err := backends.Build("claude-code", backends.Config{
		"bin":     "echo",
		"args":    "hello",
		"timeout": "5s",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer be.Close()
	resp, err := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "ignored-stdin"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(resp.Parts[0].Text)
	if got != "-p hello" {
		t.Errorf("got %q, want %q", got, "-p hello")
	}
}

func TestClaudeCode_BadTimeoutSurfaces(t *testing.T) {
	_, err := backends.Build("claude-code", backends.Config{"timeout": "nope"})
	if err == nil {
		t.Fatal("expected error on bad timeout")
	}
}

// TestClaudeCode_LongTimeoutDefault: the default timeout (5m) should
// be longer than the bare exec backend default (60s). Verify by
// building with no timeout override and confirming a 90s sleep
// doesn't error from the wrapper's side. We don't actually wait —
// we just confirm building succeeds with no override (no error means
// the default parsed cleanly).
func TestClaudeCode_DefaultTimeoutAccepted(t *testing.T) {
	be, err := backends.Build("claude-code", backends.Config{"bin": "true"})
	if err != nil {
		t.Fatalf("default config should build: %v", err)
	}
	be.Close()
}
