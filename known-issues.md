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
**Status:** Resolved (PR: `feat/cli-workspace-commands`)  
**Severity:** Critical

### Resolution
`cmd/molecule/main.go` is implemented with cobra root command, global flags
(`--api-url`, `--verbose`, `--output`), and `workspace list/get` subcommands.
Global flags are wired to `MOLECULE_API_URL` env var. CLI is runnable with
`go build -o bin/molecule ./cmd/molecule && ./bin/molecule --help`.

Still needed for full completion: workspace create/delete, agent list/inspect,
config file support, unit tests.

---

## KI-002 — No API client; all commands will make raw HTTP calls

**File:** `cmd/molecule/` (no API client package yet)  
**Status:** Partially resolved (basic client in `internal/client/platform.go`)  
**Severity:** High

### Resolution
`internal/client/platform.go` provides a thin `Platform` client with
`ListWorkspaces()` and `GetWorkspace(id)` using only the standard library.
`Agent` methods and write operations (create/delete) still need to be added.
The client is used by `workspace list` and `workspace get` commands.

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
**Status:** Resolved (PR: `feat/cli-workspace-commands`)  
**Severity:** Medium

### Resolution
Fixed the CI workflow build path: `./cmd/molecli` → `./cmd/molecule`
(matching the actual package at `cmd/molecule/main.go`). Also corrected
`go-version: '1.25'` → `go-version: '1.23'` and release artifact pattern
`molecli-*` → `molecule-*` to match the binary name. Still needs
`go mod tidy` run in CI to keep `go.sum` clean.

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
