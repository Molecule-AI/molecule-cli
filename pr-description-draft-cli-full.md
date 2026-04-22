# PR: feat/cli-full-command-tree → molecule-cli

## Status
- Branch: `feat/cli-full-command-tree` (pushed locally, GH_TOKEN dead — cannot push to origin)
- Existing PR #3 (`feat/cli-workspace-commands`) is superseded by this branch
- When token recovers: push branch, close #3, open this PR

## PR Title
feat(cli): implement full CLI command tree

## PR Body

## Summary
Implements the full CLI command tree for molecule-cli, the primary
user-facing tool for the Molecule AI platform.

- `cmd/molecule/main.go`: entry point calling `cmd.Execute()`
- `internal/cmd/root.go`: cobra root with global flags (`--api-url`,
  `--verbose`, `--output`, `--config`), registers all 4 command groups
- `internal/cmd/workspace.go`: 7 subcommands (list, create, inspect,
  delete, restart, audit, delegate)
- `internal/cmd/agent.go`: 4 subcommands (list, inspect, send, peers)
- `internal/cmd/platform.go`: 2 subcommands (audit, health)
- `internal/cmd/config.go`: 5 subcommands (list, get, set, init, view)
- `internal/cmd/http.go`: `runHTTP` helper shared by agent send and
  workspace delegate
- `internal/client/platform.go`: control plane HTTP client with
  workspace/agent/health/audit operations

All 18 subcommands wire to the platform API via `MOLECULE_API_URL`.
Binary builds cleanly to `./bin/mol` with Go 1.23.

Resolves: KI-001 (entry point), KI-002 (partial — API client), KI-003 (go.sum).

## Test plan
- [ ] `go build -o bin/mol ./cmd/molecule` — BUILD OK
- [ ] `bin/mol --help` — shows all 4 command groups
- [ ] `bin/mol workspace --help` — shows all 7 workspace subcommands
- [ ] `bin/mol agent --help` — shows all 4 agent subcommands
- [ ] `bin/mol config --help` — shows all 5 config subcommands
- [ ] `bin/mol platform --help` — shows audit, health
- [ ] `bin/mol --version` — shows version
- [ ] `MOLECULE_API_URL=http://localhost:8080 bin/mol workspace list` — hits API
- [ ] `bin/mol workspace list --output json` — structured output works
- [ ] Run CI (go test ./..., go build)

🤖 Generated with [Claude Code](https://claude.ai/claude-code)