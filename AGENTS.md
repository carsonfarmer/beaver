# Beaver — Agent Guidelines

## What is this?

Beaver is a minimal ACP (Agent Client Protocol) agent in Go. It bridges LLM
providers (via Fantasy) with ACP clients (like acpx or the bundled `bvr` CLI)
over stdio or HTTP. The goal is simplicity — the least code that does the job well.

## Design Principles

- **Simple over clever.** Don't add abstractions for "future flexibility."
- **Append-only event chain.** Session state is derived by replaying ACP
  `SessionUpdate` events through a projection. No full-session serialization.
- **Events ARE ACP.** The archive stores `acp.SessionUpdate` values directly —
  no custom event types.
- **Parent pointers, not file order.** Replay follows event parents, which can
  cross sessions. Forks and rewinds are ordinary parent-choice consequences,
  not special cases.
- **Agent owns session lifecycle.** The Agent implements SessionCreator,
  SessionLoader, SessionLister, SessionForker, and SessionResumer directly.
- **Config-driven multi-provider.** Models specified as `"provider/model"`.
  Any variant can have a custom base URL. Per-model variant overrides supported.
- **Client does the work.** File reads, writes, and terminal execution are all
  proxied to the ACP client. The agent never touches project files directly.
- **Use library types.** Don't duplicate interfaces or types that exist in
  dependencies (acp-go, fantasy).

## Package Layout

```
cmd/beaver/          — main() wiring, stdio/HTTP connection setup
cmd/bvr/             — standalone CLI client (stdio spawn or HTTP)
pkg/agent/           — ACP agent (agent.go: core + misc handlers; lifecycle.go: session lifecycle; prompt.go: Prompt + Cancel; logclient.go: SessionUpdate interceptor)
pkg/storage/         — append-only event archive (Archive interface, FileArchive/MemArchive impls, Lineage traversal)
pkg/llm/             — Registry, model resolution, provider options, type conversions
pkg/session/         — session.State (runtime), Project(events) reducer
pkg/instructions/    — AGENTS.md + Skills discovery, system prompt (takes acp.Client directly)
pkg/tools/           — Fantasy agent tools (read_file, write_file, execute, plan)
pkg/client/          — ACP client impl with real filesystem/terminal/permissions
```

## Key Types

- `llm.ModelRegistry` interface — `ResolveModel`, `ModelOptions`, `Defaults`
- `llm.Registry` — implements ModelRegistry, loaded via `LoadRegistry(path)`
- `session.State` — runtime state: cwd, model, thoughtLevel, history, usage
- `session.Project(events)` — replays a storage event chain into `*session.State`
- `storage.Archive` — catalog + append log interface (Create/Delete/List/Append/Tip/Events)
- `storage.FileArchive` / `storage.MemArchive` — durable JSONL and in-memory impls
- `storage.EventID` — `{Session, N}`; parents may cross sessions for zero-copy forks/rewinds
- `storage.Event` — immutable node (Parent + either `Info` header or `Update` payload)
- `storage.Lineage(archive, tip)` — walks parent pointers across sessions, returns chronological chain
- `agent.LoggingClient` — wraps `acp.Client`, appends non-chunk SessionUpdates to archive
- `agent.Agent` — implements `acp.Agent` + session lifecycle interfaces
- `instructions.Instructions` — `Discover(ctx, cwd, client, sid)` builds system prompt via the ACP client
- `instructions.Skill` — name, description, location parsed from SKILL.md

## Building

```bash
go build -o bin/beaver ./cmd/beaver
go build -o bin/bvr ./cmd/bvr
```

Binaries go in `bin/` (gitignored).

## Testing

- Run tests: `go test ./...`
- Run with race detector: `go test -race ./...`
- **Always run smoke tests** after changes that affect the agent, session
  lifecycle, streaming, or tool calls:
  ```bash
  bin/bvr bin/beaver sessions new
  bin/bvr bin/beaver sessions list
  bin/bvr -s <id> bin/beaver set model openai/gpt-4.1-mini
  bin/bvr --timeout 30 bin/beaver "What is 2+2?"
  ```
  Or use `acpx` with the `.acpxrc.json` config for end-to-end verification.
- Config lives in `.beaver/config.json`, session logs in `.beaver/sessions/`
- Test files should correspond to implementation files (e.g. `config_test.go`
  for `config.go`)

## What NOT to Do

- Don't add abstractions for "future flexibility" — wait for the need
- Don't read/write project files directly — always proxy through ACP client
- Don't bypass git hooks (`--no-verify`)
- Don't define interfaces that duplicate what dependencies provide
- Don't create custom event types — use `acp.SessionUpdate` variants
- **Never write migration code or backward-compatibility shims.** No format
  version checks, no "read old shape, convert to new," no dual-read paths, no
  deprecated-field aliases. When a format or type changes, the new code reads
  only the new shape. Old data is discarded. This project has no users yet;
  treat every breaking change as free.
