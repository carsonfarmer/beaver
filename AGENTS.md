# Beaver — Agent Guidelines

## What is this?

Beaver is a minimal ACP (Agent Client Protocol) agent in Go. It bridges LLM
providers (via Fantasy) with ACP clients (like acpx) over stdio. The goal is
simplicity — the least code that does the job well.

## Design Principles

- **Simple over clever.** File-backed session store, not event sourcing.
  Full session state is serialized to a single JSON file on every write.
- **Agent owns session lifecycle.** The Agent implements SessionCreator,
  SessionLoader, SessionLister, SessionForker, and SessionResumer directly.
- **Config-driven multi-provider.** Models specified as `"provider/model"`.
  Any variant can have a custom base URL. Per-model variant overrides supported.
- **Client does the work.** File reads, writes, and terminal execution are all
  proxied to the ACP client. The agent never touches project files directly.
- **Use library types.** Don't duplicate interfaces or types that exist in
  dependencies (acp-go, fantasy). Use `acp.SessionStore`, `acp.MemoryStore`, etc.
- **No speculative abstractions.** Three similar lines beat a premature helper.

## Package Layout

```
cmd/beaver/          — main() wiring, stdio connection setup
pkg/agent/           — ACP agent, session lifecycle, acpFilesystem adapter
pkg/llm/             — Registry, model resolution, provider options, type conversions
pkg/session/         — Session struct, FileStore (JSON file per session)
pkg/instructions/    — AGENTS.md + Skills discovery, Filesystem interface, system prompt
pkg/tools/           — Fantasy agent tools (read_file, write_file, execute), RunCommand
```

## Key Types

- `llm.ModelRegistry` interface — `ResolveModel`, `ModelOptions`, `Defaults`
- `llm.Registry` — implements ModelRegistry, loaded via `LoadRegistry(path)`
- `session.Session` — cwd, model, thoughtLevel, context, skills, history, cancel
- `session.FileStore` — implements `acp.SessionStore[*Session]`
- `agent.Agent` — implements `acp.Agent` + session lifecycle interfaces
- `instructions.Filesystem` — `ReadFile`, `WriteFile`, `ListDir`
- `instructions.Skill` — name, description, location parsed from SKILL.md

## Testing

- Run tests: `go test ./...`
- Run with race detector: `go test -race ./...`
- Build: `go build -o /tmp/beaver ./cmd/beaver`
- **Always run smoke tests via acpx** after changes that affect the agent,
  session lifecycle, streaming, or tool calls. Build the binary and use
  `acpx` with the `.acpxrc.json` config to verify end-to-end behavior.
- Config lives in `.beaver/config.json`, sessions in `.beaver/sessions/`
- Test files should correspond to implementation files (e.g. `config_test.go`
  for `config.go`, not a single `pkg_test.go`)

## What NOT to Do

- Don't add event sourcing, JSONL logs, or middleware-based persistence
- Don't add abstractions for "future flexibility" — wait for the need
- Don't read/write project files directly — always proxy through ACP client
- Don't bypass git hooks (`--no-verify`)
- Don't define interfaces that duplicate what dependencies provide
