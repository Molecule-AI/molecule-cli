// Package exec implements the `exec` backend: each inbound A2A
// message is dispatched to a configured shell command. The text parts
// are written to the subprocess's stdin; stdout becomes the reply.
//
// This is the most general external-bridge: any handler that can read
// stdin and write stdout works. Claude Code (`claude -p`), `ollama
// run <model>`, custom Python scripts, etc.
//
// Config keys:
//   - cmd       (required): shell command, e.g. "claude -p" or
//     "python myhandler.py". Run via /bin/sh -c on Unix or cmd /c on
//     Windows so quoting + pipes + env-var expansion work as users
//     expect from a terminal.
//   - timeout   (optional): per-message timeout duration string
//     (Go time.ParseDuration), default "60s". The subprocess is killed
//     on timeout and the backend returns an error so the dispatcher
//     keeps the message in the activity queue for re-delivery on a
//     later run.
//   - pass_meta (optional): when "true", populate the subprocess env
//     with MOLECULE_WORKSPACE_ID, MOLECULE_CALLER_ID, MOLECULE_MESSAGE_ID,
//     MOLECULE_TASK_ID, MOLECULE_METHOD. Useful for handlers that
//     thread context across messages.
//
// Concurrency: HandleA2A is safe to call concurrently; each call
// spawns its own subprocess. The dispatcher serializes calls within a
// poll batch, so in practice there is at most one subprocess running.
//
// Security note: cmd runs through sh -c, which means the operator's
// command line is the trust boundary. Don't pass user-controlled
// strings into cmd. The inbound message text goes via stdin, not
// argv, so a malicious sender can't inject shell metacharacters.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Molecule-AI/molecule-cli/internal/backends"
)

func init() {
	backends.Register("exec", New)
}

// New builds an exec backend from cfg. The cmd key is required; other
// keys default sensibly.
func New(cfg backends.Config) (backends.Backend, error) {
	cmd, err := cfg.Require("cmd")
	if err != nil {
		return nil, fmt.Errorf("exec backend: %w", err)
	}
	timeoutStr := cfg.Get("timeout", "60s")
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return nil, fmt.Errorf("exec backend: parse timeout %q: %w", timeoutStr, err)
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("exec backend: timeout must be positive, got %s", timeoutStr)
	}
	passMeta := strings.EqualFold(cfg.Get("pass_meta", "false"), "true")

	return &Backend{
		cmd:      cmd,
		timeout:  timeout,
		passMeta: passMeta,
	}, nil
}

// Backend is the exec implementation. Stateless across messages — each
// call spawns a fresh subprocess.
type Backend struct {
	cmd      string
	timeout  time.Duration
	passMeta bool
}

// HandleA2A spawns the configured command, pipes the joined text parts
// to stdin, captures stdout, and returns it as the reply. Stderr is
// captured separately and surfaced in the error message on failure so
// the operator can see what their command printed.
func (b *Backend) HandleA2A(ctx context.Context, req backends.Request) (backends.Response, error) {
	input := joinTextParts(req.Parts)

	runCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	shell, shellArg := platformShell()
	cmd := osexec.CommandContext(runCtx, shell, shellArg, b.cmd)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if b.passMeta {
		cmd.Env = append(os.Environ(),
			"MOLECULE_WORKSPACE_ID="+req.WorkspaceID,
			"MOLECULE_CALLER_ID="+req.CallerID,
			"MOLECULE_MESSAGE_ID="+req.MessageID,
			"MOLECULE_TASK_ID="+req.TaskID,
			"MOLECULE_METHOD="+req.Method,
		)
	}

	err := cmd.Run()
	// Always surface stderr if the command produced any — operators
	// rely on stderr for log lines even on success.
	stderrTail := tail(stderr.String(), 1024)
	if err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return backends.Response{}, fmt.Errorf("exec backend: command %q timed out after %s (stderr: %s)",
				b.cmd, b.timeout, stderrTail)
		}
		return backends.Response{}, fmt.Errorf("exec backend: command %q failed: %w (stderr: %s)",
			b.cmd, err, stderrTail)
	}

	return backends.TextResponse(stdout.String()), nil
}

// Close is a no-op — exec spawns subprocesses on-demand, nothing to
// release at shutdown beyond what the OS already does when the parent
// exits.
func (b *Backend) Close() error { return nil }

// joinTextParts concatenates the text parts of a request, ignoring
// data/file parts. Text-only is the M1 contract; richer marshalling
// (e.g. JSON-on-stdin for backends that want full structure) is a
// future opt-in via a `format` config key.
func joinTextParts(parts []backends.Part) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Type == "text" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// platformShell returns the shell binary + the "run this command
// string" argument for the current OS. On Windows, cmd.exe uses /c;
// everywhere else, /bin/sh -c works.
func platformShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "cmd.exe", "/c"
	}
	return "/bin/sh", "-c"
}

// tail returns the last n bytes of s, prefixed with "..." if truncated.
// Used to keep stderr quotes in error messages bounded.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
