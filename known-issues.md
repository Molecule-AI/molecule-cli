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
**Status:** Not yet implemented  
**Severity:** Critical

### Symptom
The repo is initialized as a Go module but has no `cmd/molecule/main.go`. Running
`go build ./cmd/molecule` or `go run ./cmd/molecule` fails with
"package cmd/molecule: cannot find module" or "build failed".

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
**Status:** Not yet implemented  
**Severity:** High

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
**Status:** Identified  
**Severity:** Low

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

**File:** `.github/workflows/release.yml`  
**Status:** Not verified  
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

**File:** `tests/` (does not exist)  
**Status:** Not yet implemented  
**Severity:** Medium

### Symptom
There are no tests at all (per `go test ./...` — no packages match).
As subcommands are built, there is no test harness for end-to-end CLI testing
(e.g. `molecule workspace create --name test --output json` → verify JSON output).

### Impact
Each subcommand will be shipped without regression protection. Manual testing
is required for every release. The absence of a `tests/` directory also means
there is no fixture for CLI integration testing with recorded API responses.

### Suggested fix
Add `tests/` with:
- `cmd/molecule/molecule_test.go` — table-driven tests for each subcommand
  using `exec.Command("molecule", ...)` against a built binary
- Use `molecule-sdk-python` fixture server or recorded API responses for
  offline testing
- Add `go test ./...` to CI; require >0 test packages before merge
