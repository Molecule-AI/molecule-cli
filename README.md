# molecule-cli

Command-line companion for [Molecule AI](https://moleculesai.com). The
primary entry point for **external-runtime workspaces** — bridge a
workspace to a local agent backend (Claude Code, an arbitrary shell
command, or a mock for CI).

## Install

```bash
go install github.com/Molecule-AI/molecule-cli/cmd/molecule@latest
```

Or download a binary from [Releases](https://github.com/Molecule-AI/molecule-cli/releases).
Releases ship Linux/macOS/Windows × amd64/arm64 archives plus a sha256
checksums file (see `.goreleaser.yaml`).

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
--mode poll|push         delivery mode (default: poll; push is M4, not yet implemented)
--interval-ms 2000       poll cadence
--since-secs 60          initial activity cursor lookback
--token TOKEN            override MOLECULE_WORKSPACE_TOKEN
--dry-run                build backend + print summary, exit
```

State (the activity cursor) is persisted to
`~/.config/molecule/state/<workspace-id>.json` (mode 0600) so a restart
resumes from the last delivered message.

> Note: poll-mode dispatch into backends works end-to-end. Posting the
> backend's reply back to the canvas-origin sender is still wired as a
> TODO (see `internal/connect/connect.go` — M1.3); platform-API replies
> for non-canvas A2A flow correctly.

## Command reference

The full top-level tree:

```
molecule
├── workspace    list / create / inspect / delete / restart / audit / delegate
├── agent        list / inspect / send / peers
├── platform     audit / health
├── config       list / get / set / init / view
├── connect      bridge an external workspace to a local backend
├── init         scaffold a molecule.yaml in the current directory
└── completion   emit shell completion script (bash | zsh | fish | powershell)
```

Global flags (apply to every subcommand): `--api-url`, `--config`,
`-o/--output table|json|yaml`, `-v/--verbose`.

### `molecule workspace`

Manage Molecule AI workspaces.

- **`workspace list`** — list all workspaces in the org.
  ```bash
  molecule workspace list
  molecule workspace list -o json
  ```
- **`workspace create --name <name> [flags]`** — create a workspace.
  Flags: `--role`, `--runtime`, `--template`, `--parent-id`,
  `--workspace-dir`, `--tier`.
  ```bash
  molecule workspace create --name pm-bot --role pm --runtime claude-code
  ```
- **`workspace inspect <workspace-id>`** — show full details for a workspace.
  ```bash
  molecule workspace inspect ws_01HF2K...
  ```
- **`workspace delete <workspace-id>`** — delete a workspace (irreversible).
  ```bash
  molecule workspace delete ws_01HF2K...
  ```
- **`workspace restart <workspace-id>`** — trigger a restart.
  ```bash
  molecule workspace restart ws_01HF2K...
  ```
- **`workspace audit`** — workspaces + agents report grouped by status.
  ```bash
  molecule workspace audit -o yaml
  ```
- **`workspace delegate <workspace-id> <target-workspace-id> <task>`** —
  enqueue a non-blocking delegation from one workspace to another.
  ```bash
  molecule workspace delegate ws_pm ws_research "summarize last week"
  ```

### `molecule agent`

Inspect and interact with agents.

- **`agent list [workspace-id]`** — list all agents, optionally scoped
  to one workspace.
  ```bash
  molecule agent list
  molecule agent list ws_01HF2K...
  ```
- **`agent inspect <agent-id>`** — show full details for an agent.
  ```bash
  molecule agent inspect agent_abc
  ```
- **`agent send <agent-id> <message>`** — send a one-shot A2A message
  to an agent and print the reply.
  ```bash
  molecule agent send agent_abc "what's the deploy status?"
  ```
- **`agent peers <workspace-id>`** — list peer workspaces reachable
  from the given workspace.
  ```bash
  molecule agent peers ws_01HF2K...
  ```

### `molecule platform`

Platform-level operations.

- **`platform audit`** — full audit: workspaces, agents, delegation
  counts per workspace.
  ```bash
  molecule platform audit -o json
  ```
- **`platform health`** — check `/health` and version (falls back to
  raw probe on older platforms).
  ```bash
  molecule platform health
  ```

### `molecule config`

View and manage CLI configuration. Values resolve in order: env vars >
config file > defaults.

- **`config list`** — list all known config keys + effective values + source.
- **`config get <key>`** — print a single config value.
- **`config set <key> <value>`** — write a key to `~/.config/molecule.yaml`.
- **`config init`** — scaffold a default `molecule.yaml` in the current dir.
- **`config view`** — print the active config file with annotations.

```bash
molecule config set api_url https://your-tenant.staging.moleculesai.app
molecule config get api_url
molecule config list
```

### `molecule connect`

See [Quick start](#quick-start--connect-an-external-workspace) above.
Push mode (`--mode push`) is reserved for M4 and currently exits with a
clear error — use the default `--mode poll`.

### `molecule init`

Bootstrap a workspace by scaffolding `molecule.yaml` in the current
directory. Use `--force` to replace an existing file.

```bash
molecule init
molecule init --force
```

### `molecule completion`

Emit a shell completion script. The shell name is a positional arg —
one of `bash`, `zsh`, `fish`, `powershell`.

```bash
# bash
source <(molecule completion bash)

# zsh
source <(molecule completion zsh)

# fish
molecule completion fish | source

# PowerShell
molecule completion powershell | Out-String | Invoke-Expression
```

The full M1 design is in [RFC #10](https://github.com/Molecule-AI/molecule-cli/issues/10).

## License

Business Source License 1.1 — © Molecule AI.
