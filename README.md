# beaver

A minimal [ACP](https://agentclientprotocol.org) agent in Go, built on:

- [`ironpark/go-acp`](https://github.com/ironpark/go-acp) for the ACP protocol
- [`charm.land/fantasy`](https://github.com/charmbracelet/fantasy) for LLM orchestration and streaming

## Features

- ACP server over stdio
- Multi-provider LLM support (OpenAI, Anthropic, Google, OpenRouter, OpenAI-compatible)
- Full session lifecycle: create, load, list, fork, resume
- Session persistence — full state saved as JSON per session
- [Agent Skills](https://agentskills.io) support — discovered on session creation
- Streaming text and reasoning deltas via `session/update`
- Tool proxying to ACP client:
  - `read_file` → `fs/read_text_file`
  - `write_file` → `fs/write_text_file`
  - `execute` → `terminal/create` + wait + output + release
- Configurable model and thought level via `session/set_config_option`
- OpenAI Responses API enabled by default

## Configuration

Beaver reads `.beaver/config.json`:

```json
{
  "providers": {
    "openai": {
      "variant": "openai",
      "env": ["OPENAI_API_KEY"],
      "models": {
        "gpt-4.1-mini": { "name": "GPT-4.1 Mini" },
        "gpt-4.1": { "name": "GPT-4.1" }
      }
    },
    "anthropic": {
      "variant": "anthropic",
      "env": ["ANTHROPIC_API_KEY"],
      "models": {
        "claude-sonnet-4-5-20250514": { "name": "Claude Sonnet 4.5" }
      }
    }
  },
  "defaults": {
    "model": "openai/gpt-4.1-mini",
    "thoughtLevel": "medium"
  }
}
```

Models are specified as `"provider/model"`. Any variant can have a custom base
URL via the `"api"` field. Per-model variant overrides are supported.

## Skills

Beaver supports [Agent Skills](https://agentskills.io). Place skills in
`.agents/skills/` (project-level) or `~/.agents/skills/` (user-level):

```
.agents/skills/
└── my-skill/
    └── SKILL.md
```

Skills are discovered on session creation. The model sees available skill names
and descriptions, and can load full instructions via `read_file` when relevant.

## Run

```bash
go build -o bin/beaver ./cmd/beaver
```

## Usage with acpx

```bash
# Create a project-level .acpxrc.json (see below), then:
acpx beaver sessions new --name my-session
acpx --approve-all beaver -s my-session "hello from acpx"

# Resume after restart
acpx beaver -s my-session "what did I say before?"
```

Example `.acpxrc.json`:

```json
{
  "defaultAgent": "beaver",
  "agents": {
    "beaver": {
      "command": "./bin/beaver -data .beaver"
    }
  }
}
```

## Test

```bash
go test ./...
```

## Project Structure

```
cmd/beaver/      main() — wiring, stdio connection
pkg/llm/         config, model resolution, ACP↔Fantasy conversions
pkg/session/     Session struct, file-backed session store
pkg/agent/       ACP agent, session lifecycle, tool definitions
pkg/skills/      Agent Skills parsing and prompt generation
```

## License

MIT
