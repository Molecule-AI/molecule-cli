# molecule-cli

Command-line companion for [Molecule AI](https://moleculesai.com). The
primary entry point for **external-runtime workspaces** — bridge a
workspace to a local agent backend (Claude Code, an arbitrary shell
command, or a mock for CI).

## Install

> **Migration in progress** (2026-05-07): the `github.com/Molecule-AI`
> org was suspended on 2026-05-06 and is permanently gone. The
> canonical SCM is now Gitea at
> [`git.moleculesai.app/molecule-ai`](https://git.moleculesai.app/molecule-ai).
>
> The `go install` path below requires the Go module-path migration to
> land first (separate cross-repo PR — see `internal#37` parked
> follow-up). Until then, build from source:
>
> ```bash
> git clone https://git.moleculesai.app/molecule-ai/molecule-cli.git
> cd molecule-cli
> go build -o molecule ./cmd/molecule
> ```
>
> Once the Go module-path migration lands:

```bash
go install git.moleculesai.app/molecule-ai/molecule-cli/cmd/molecule@latest
```

Releases ship Linux/macOS/Windows × amd64/arm64 archives plus a sha256
checksums file (see `.goreleaser.yaml`). Releases pipeline (Gitea
Actions) is being restored as part of the post-suspension recovery; in
the interim, build from source per the migration note above.

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
