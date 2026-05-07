// Package claudecode implements the `claude-code` backend: a thin
// shorthand for `exec` with `cmd="claude -p"`. Each inbound A2A
// message is dispatched to a fresh `claude --print` invocation; the
// model's stdout becomes the reply.
//
// Why a separate backend instead of telling users `--backend exec
// --backend-opt cmd="claude -p"`?
//
//   - The default backend is "claude-code" — copy-paste-from-canvas
//     should Just Work without the operator memorising flag spelling.
//   - Future versions can add Claude-Code-specific config: model
//     selection, system prompt, MCP forwarding. The exec backend
//     stays generic.
//
// Config keys (all optional):
//
//   - bin       — claude binary path. Default "claude".
//   - args      — extra args appended after `-p`. Default "".
//   - timeout   — per-message timeout. Default "5m" (Claude Code
//     responses can take a while; longer than exec's 60s default).
//   - pass_meta — see exec backend. Default "true" — Claude Code
//     sessions benefit from knowing who sent the message.
//
// Implementation: builds the equivalent exec config under the hood.
// Reusing exec means timeout/stdin/stderr/env handling stays in one
// place; bug fixes flow to both.
package claudecode

import (
	"strings"

	"go.moleculesai.app/cli/internal/backends"
	exec "go.moleculesai.app/cli/internal/backends/exec"
)

func init() {
	backends.Register("claude-code", New)
}

// New builds a claude-code backend from cfg. Translates the
// claude-code keys to an exec backend config and delegates.
func New(cfg backends.Config) (backends.Backend, error) {
	bin := cfg.Get("bin", "claude")
	extra := cfg.Get("args", "")
	timeout := cfg.Get("timeout", "5m")
	passMeta := cfg.Get("pass_meta", "true")

	// Build the underlying shell command. -p (print mode) is the
	// non-interactive Claude Code mode that reads stdin and writes
	// stdout once.
	cmd := bin + " -p"
	if strings.TrimSpace(extra) != "" {
		cmd += " " + extra
	}

	return exec.New(backends.Config{
		"cmd":       cmd,
		"timeout":   timeout,
		"pass_meta": passMeta,
	})
}
