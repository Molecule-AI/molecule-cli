# molecule-cli (molecli)

Go TUI dashboard for Molecule AI — real-time workspace monitoring, event log, health overview, delete/filter operations.

## Install

```bash
go install github.com/Molecule-AI/molecule-cli/cmd/molecli@latest
```

Or download a binary from [Releases](https://github.com/Molecule-AI/molecule-cli/releases).

## Usage

```bash
export MOLECLI_URL=http://localhost:8080  # or your platform URL
molecli
```

## Features

- Real-time workspace status (online/offline/degraded/paused)
- Event log with filtering
- Workspace CRUD operations
- Agent session management
- Memory/skill inspection
- A2A chat interface

## License

Business Source License 1.1 — © Molecule AI.
