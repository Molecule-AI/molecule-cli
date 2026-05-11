# Known Issues — molecule-cli

Issues identified in source but not yet filed as GitHub issues (GH_TOKEN
unavailable in automated agent contexts). Each entry has: location,
symptom, impact, suggested fix.

Format per entry:
```
## KI-N — Short title

**File:** `<path>:<line>`
**Status:** TODO comment / identified / partially fixed
**Severity:** Critical / High / Medium / Low

### Symptom
...

### Impact
...

### Suggested fix
...
---
```

---

## KI-001 — No entry point yet (`cmd/molecule/main.go` does not exist)

**File:** `cmd/molecule/main.go`
**Status:** ✅ Resolved
**Resolved in:** `feat/cli-full-command-tree` branch, commit "feat: implement full CLI command tree"

### Symptom
`cmd/molecule/main.go` exists and calls `cmd.Execute()`. Root command is wired
with global flags (`--verbose`, `--output`, `--config`, `--api-url`). All
subcommand groups registered: workspace (7 commands), agent (4 commands),
platform (2 commands), config (5 commands). Binary builds to `bin/mol`.

### Impact
The CLI is not runnable. No workspace management, agent inspection, or any other
CLI command exists. The repo is a stub.

### Suggested fix
Implement `cmd/molecule/main.go` with a root `cobra.Command` that registers
subcommands. Wire up global flags (`--verbose`, `--output`, `--config`).
Wire `MOLECULE_API_URL` env var as the default for the API base URL.
See the stub checklist in `CLAUDE.md` Section 8.

---

## KI-002 — No API client; all commands will make raw HTTP calls

**File:** `cmd/molecule/` (no API client package yet)
**Status:** ✅ Partially resolved
**Resolved in:** `internal/client/platform.go` exists with workspace and agent
operations; `runHTTP` helper in `internal/cmd/http.go` used by `agent send` and
`workspace delegate`. Remaining: workspace runtime client (dev/proxy mode).

### Symptom
There is no `internal/client/` or `pkg/api/` package. Any subcommand
implementation will need to import the platform SDK (`molecule-sdk-python`) via
a Go FFI wrapper, make raw `net/http` calls directly, or wait for a Go SDK to be
built. Neither exists yet.

### Impact
Subcommand implementations will either duplicate HTTP client logic or require
architecting a clean API client interface before the first command can be
meaningfully built.

### Suggested fix
Before implementing subcommands, define `internal/api/client.go` with a
`Client` struct wrapping `*http.Client`. Implement methods for workspace and
agent operations. Add a `ClientOption` functional options pattern for
configuring base URL and auth. Document the API endpoints in `docs/` as they
are implemented.

---

## KI-003 — `go.sum` may contain entries from non-release toolchains

**File:** `go.sum`
**Status:** ✅ Resolved
**Resolved in:** `go mod tidy` run on `feat/cli-full-command-tree`; `go.sum` regenerated
clean. Dependencies: cobra v1.10.2, viper v1.21.0, their transitive deps.

### Symptom
The `go.sum` file was generated during initial module setup. It may contain
checksum entries for transitive dependencies pulled from toolchains or
platforms not intended for the release build (e.g. `linux/arm64` on an `amd64`
host). GoReleaser targets specific platforms and any spurious `go.sum`
entries may cause CI divergence or checksum mismatches.

### Impact
`go mod verify` in CI may fail if `go.sum` has extra entries not in the
 lock file. Additionally, if the module path (`go.moleculesai.app/cli`)
 is referenced via `replace` directives from other repos, those references may
persist stale entries.

### Suggested fix
Run `go mod tidy` on a clean checkout from `main` and commit only the
resulting `go.sum`. Add `go mod verify` to CI as a lint step. Ensure
`.goreleaser.yaml` specifies exact Go version matching CI.

---

## KI-004 — GoReleaser config may not be aligned with go.mod module path

**File:** `.github/workflows/release.yml`
**Status:** ⚠️ Unverified — needs real tag to confirm
**Severity:** Medium

### Symptom
The GoReleaser workflow is wired up but has not been tested with a real tag.
The `gomod.alphaSettings` or `builds[].dir` settings in `.goreleaser.yaml`
(if it exists) may not correctly resolve the module root. A real `v*` tag
push could produce an empty release or a binary with the wrong name.

### Impact
The first release may silently fail or produce a malformed artifact that is
not usable by platform operators.

### Suggested fix
Before the first release, test goreleaser locally with `goreleaser check`
and `goreleaser snapshot --clean`. Verify the binary name, module path, and
target OS/arch match expectations. Ensure `goreleaser.yaml` `builds[].dir`
is set to `.` (repo root) since the main package is at `cmd/molecule`.

---

## KI-005 — No integration test for the full CLI lifecycle

**File:** `cmd/molecule/molecule_test.go`, `internal/`  
**Status:** ✅ Resolved  
**Severity:** Medium

### Resolution
`cmd/molecule/molecule_test.go` contains 32 table-driven integration tests
covering the full CLI command surface. Each test:
- Starts a `httptest.Server` that mirrors the platform REST API (workspace CRUD,
  agent inspection, delegation, health, audit, config, completion scripts).
- Builds the `molecule` binary via `go build -o <tempdir>/molecule .`
- Executes it with `exec.Command` and validates stdout/stderr output.

Test coverage includes:
- Root help, workspace/agent/platform/config help
- `workspace list`, `workspace inspect`, `workspace create`, `workspace delete`,
  `workspace restart`, `workspace audit`, `workspace delegate`
- `agent list`, `agent inspect`, `agent send`, `agent peers`
- `platform health`, `platform audit`
- `config init`, `config list`
- `init`, `init --force`
- `completion bash/zsh/fish/powershell`
- Error paths: missing workspace, missing agent, unknown subcommand,
  missing required args, duplicate init

Run `go test ./...` from the repo root to execute. No live platform required —
all tests use `httptest.Server` fixtures.
