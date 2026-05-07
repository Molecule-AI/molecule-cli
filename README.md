# molecule-cli

Command-line companion for [Molecule AI](https://moleculesai.com). The
primary entry point for **external-runtime workspaces** — bridge a
workspace to a local agent backend (Claude Code, an arbitrary shell
command, or a mock for CI).

## Install

```bash
go install go.moleculesai.app/cli/cmd/molecule@latest
```

The vanity import path `go.moleculesai.app/cli` resolves via the
Molecules AI go-get responder (issue [internal#71][i71]) to our
canonical SCM at git.moleculesai.app. It is independent of any specific
SCM host — when we move SCMs again, no install command changes.

Alternatively, build from source:

```bash
git clone https://git.moleculesai.app/molecule-ai/molecule-cli.git
cd molecule-cli
go build -o molecule ./cmd/molecule
```

[i71]: https://git.moleculesai.app/molecule-ai/internal/issues/71

## Quick start — connect an external workspace

When you create a workspace with `runtime: external`, the platform returns
a per-workspace token. Run:

```bash
export MOLECULE_API_URL=https://your-tenant.staging.moleculesai.app
export MOLECULE_WORKSPACE_TOKEN=ws_secret_xxx

molecule connect ws_abcdef
```

`connect` registers the workspace, starts heartbeating, polls the platform
for inbound A2A messages, and dispatches each message to the selected
backend. Replies are posted back over the platform API.

### Backends

`--backend` selects how A2A messages are handled. Three are built in:

| Name          | What it does                                                                 |
|---------------|------------------------------------------------------------------------------|
| `claude-code` | Default. Invokes `claude -p <message>` for each turn (claude-code SDK).      |
| `exec`        | Runs an arbitrary shell command (`--backend-opt cmd=...`). Stdout = reply.   |
| `mock`        | Echo backend for CI / smoke tests.                                           |

Backend options are passed via repeatable `--backend-opt KEY=VALUE`:

```bash
# Claude Code with a 10-minute per-turn timeout
molecule connect ws_abc \
  --backend claude-code \
  --backend-opt timeout=10m

# Generic shell handler
molecule connect ws_abc \
  --backend exec \
  --backend-opt cmd='./my-agent.sh' \
  --backend-opt timeout=5m
```

### Other useful flags

```
--mode poll|push         delivery mode (default: poll)
--interval-ms 2000       poll cadence
--since-secs 60          initial activity cursor lookback
--token TOKEN            override MOLECULE_WORKSPACE_TOKEN
--dry-run                build backend + print summary, exit
```

State (the activity cursor) is persisted to
`~/.config/molecule/state/<workspace-id>.json` (mode 0600) so a restart
resumes from the last delivered message.

## Other subcommands

```
molecule agent      list / inspect agent sessions
molecule config     view / set CLI defaults
molecule completion generate shell completions
```

The full M1 design is in [RFC #10](https://git.moleculesai.app/molecule-ai/molecule-cli/issues/10) (originally filed on the suspended GitHub org; the issue may be re-filed on Gitea — check the issue tracker for the live discussion).

## License

Business Source License 1.1 — © Molecule AI.
