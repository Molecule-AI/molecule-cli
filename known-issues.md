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
 lock file. Additionally, if the module path (`github.com/Molecule-AI/molecule-cli`)
 is referenced via `replace` directives from other repos, those references may
persist stale entries.

### Suggested fix
Run `go mod tidy` on a clean checkout from `main` and commit only the
resulting `go.sum`. Add `go mod verify` to CI as a lint step. Ensure
`.goreleaser.yaml` specifies exact Go version matching CI.

---

## KI-004 — GoReleaser config may not be aligned with go.mod module path

**File:** `.goreleaser.yaml`
**Status:** ✅ Resolved — `.goreleaser.yaml` added
**Resolved in:** `main` (commit `47b2804` + this branch)
**Severity:** Medium

### Symptom
The GoReleaser workflow was wired up but had no `.goreleaser.yaml` config.
A `v*` tag push could produce an empty release or a binary with the wrong name
if `builds[].dir` or `builds[].main` were misconfigured.

### Resolution
Added `.goreleaser.yaml` with:
- `dir: .` — repo root
- `main: ./cmd/molecule` — main package path
- `binary: molecule` — output binary name
- All 6 targets: linux/darwin × amd64/arm64 + windows × amd64
- `CGO_ENABLED=0` for static binaries
- Checksum files generated for all archives

`release.yml` still uses plain `go build` per matrix target (GoReleaser is
configured but not wired into CI yet — the plain build is sufficient for
v0.1.0). Wire GoReleaser into CI when Homebrew formula + checksum
verification are needed.

---

## KI-005 — No integration test for the full CLI lifecycle

**File:** `tests/` (does not exist)
**Status:** ✅ Resolved
**Resolved in:** `cmd/molecule/molecule_test.go` — 24 table-driven tests using httptest mock server.
**Severity:** Medium

### Symptom
There were no tests at all (per `go test ./...` — no packages match).
As subcommands were built, there was no test harness for end-to-end CLI testing
(e.g. `molecule workspace create --name test --output json` → verify JSON output).

### Impact
Each subcommand was shipped without regression protection. Manual testing
was required for every release.

### Suggested fix
Add `tests/` with:
- `cmd/molecule/molecule_test.go` — table-driven tests for each subcommand
  using `exec.Command("molecule", ...)` against a built binary
- Use a httptest mock server for offline testing
- Add `go test ./...` to CI; require >0 test packages before merge

**✅ Done:** 24 integration tests covering all 18 subcommands, error paths,
and structured output. `go test ./...` passes, CI job added to `release.yml`.
