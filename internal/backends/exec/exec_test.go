package exec_test

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"go.moleculesai.app/cli/internal/backends"
	_ "go.moleculesai.app/cli/internal/backends/exec" // register
)

// requireUnix skips Windows tests that depend on /bin/sh shell semantics.
// Windows-shell coverage is in TestPlatformShell_Windows below.
func requireUnix(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix-only: shell command semantics differ on Windows")
	}
}

func TestExec_RequiresCmd(t *testing.T) {
	_, err := backends.Build("exec", backends.Config{})
	if err == nil {
		t.Fatal("expected error when cmd is unset")
	}
	if !strings.Contains(err.Error(), "cmd") {
		t.Errorf("error should mention cmd: %v", err)
	}
}

func TestExec_RejectsBadTimeout(t *testing.T) {
	_, err := backends.Build("exec", backends.Config{"cmd": "echo hi", "timeout": "not-a-duration"})
	if err == nil {
		t.Fatal("expected error on bad timeout")
	}
}

func TestExec_RejectsZeroTimeout(t *testing.T) {
	_, err := backends.Build("exec", backends.Config{"cmd": "echo hi", "timeout": "0s"})
	if err == nil {
		t.Fatal("expected error on zero timeout")
	}
}

func TestExec_EchoStdinToStdout(t *testing.T) {
	requireUnix(t)
	be, err := backends.Build("exec", backends.Config{"cmd": "cat"})
	if err != nil {
		t.Fatal(err)
	}
	defer be.Close()

	resp, err := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "hello world"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Parts[0].Text; got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
	if !resp.Final {
		t.Error("expected Final=true")
	}
}

func TestExec_ConcatenatesTextPartsOnly(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{"cmd": "cat"})
	defer be.Close()
	resp, _ := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{
			{Type: "text", Text: "a"},
			{Type: "data", Data: map[string]interface{}{"k": "v"}}, // ignored
			{Type: "text", Text: "b"},
		},
	})
	if got := resp.Parts[0].Text; got != "ab" {
		t.Errorf("got %q, want ab", got)
	}
}

func TestExec_NonZeroExitSurfacesStderr(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{
		"cmd": "echo my-stderr-text >&2; exit 17",
	})
	defer be.Close()
	_, err := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "x"}},
	})
	if err == nil {
		t.Fatal("expected error on exit 17")
	}
	if !strings.Contains(err.Error(), "my-stderr-text") {
		t.Errorf("error should include stderr: %v", err)
	}
}

func TestExec_TimeoutKillsRunaway(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{
		"cmd":     "sleep 5",
		"timeout": "100ms",
	})
	defer be.Close()
	_, err := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "x"}},
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should mention timeout: %v", err)
	}
}

func TestExec_PassMetaInjectsEnv(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{
		"cmd":       `printf "ws=%s caller=%s msg=%s task=%s method=%s" "$MOLECULE_WORKSPACE_ID" "$MOLECULE_CALLER_ID" "$MOLECULE_MESSAGE_ID" "$MOLECULE_TASK_ID" "$MOLECULE_METHOD"`,
		"pass_meta": "true",
	})
	defer be.Close()
	resp, err := be.HandleA2A(context.Background(), backends.Request{
		WorkspaceID: "ws-1",
		CallerID:    "ws-2",
		MessageID:   "act-3",
		TaskID:      "task-4",
		Method:      "message/send",
		Parts:       []backends.Part{{Type: "text", Text: "x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "ws=ws-1 caller=ws-2 msg=act-3 task=task-4 method=message/send"
	if got := resp.Parts[0].Text; got != want {
		t.Errorf("env injection: got %q, want %q", got, want)
	}
}

func TestExec_NoPassMetaLeavesEnvUntouched(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{
		"cmd": `printf "%s" "$MOLECULE_WORKSPACE_ID"`,
		// pass_meta unset → defaults to false
	})
	defer be.Close()
	resp, _ := be.HandleA2A(context.Background(), backends.Request{
		WorkspaceID: "ws-secret",
		Parts:       []backends.Part{{Type: "text", Text: "x"}},
	})
	if got := resp.Parts[0].Text; got != "" {
		t.Errorf("expected empty env (pass_meta off); got %q", got)
	}
}

func TestExec_ParentEnvAvailableEvenWhenPassMetaOff(t *testing.T) {
	requireUnix(t)
	// When pass_meta is off, we still inherit the parent process env
	// (cmd.Env nil → os.Environ). Verify by reading PATH.
	be, _ := backends.Build("exec", backends.Config{"cmd": `printf "%s" "$PATH"`})
	defer be.Close()
	resp, _ := be.HandleA2A(context.Background(), backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "x"}},
	})
	if !strings.Contains(resp.Parts[0].Text, os.Getenv("PATH")[:min(len(os.Getenv("PATH")), 8)]) {
		t.Errorf("expected PATH inheritance; got %q", resp.Parts[0].Text)
	}
}

// TestExec_ContextCancelKillsCommand: when the caller's ctx is
// cancelled mid-run, the subprocess is killed immediately (vs waiting
// for our internal timeout to fire).
func TestExec_ContextCancelKillsCommand(t *testing.T) {
	requireUnix(t)
	be, _ := backends.Build("exec", backends.Config{
		"cmd":     "sleep 5",
		"timeout": "30s",
	})
	defer be.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, err := be.HandleA2A(ctx, backends.Request{
		Parts: []backends.Part{{Type: "text", Text: "x"}},
	})
	if err == nil {
		t.Fatal("expected error on cancelled ctx")
	}
}

// Tiny helper since math/min is generic since Go 1.21.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
