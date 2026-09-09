# Vohu

[![Go](https://github.com/Aliizi83/vohu/actions/workflows/go.yml/badge.svg)](https://github.com/Aliizi83/vohu/actions/workflows/go.yml)

A provider-independent AI agent runtime for Go — built from scratch to run tool-calling agent loops safely.

> **From thought to action.**

## Status

🚧 Early stage, actively developed. APIs and internals may change.

## What is Vohu

Vohu connects to an LLM, lets the model call tools, executes those tools locally, and feeds the results back — a real agent loop (`user → model → tool call → tool execution → tool result → model → final response`), not a one-shot chat wrapper.

Two design goals shape everything in the codebase:

- **Provider-independent.** The agent loop, tool registry, and conversation history know nothing about "Gemini" or "Anthropic". Providers plug in behind a single `LLM` interface, so adding a new one never touches the loop or the tools.
- **Safe by default.** Any tool that can affect the host system — starting with command execution — sits behind an explicit, evaluated-before-execution security policy, not a best-effort guardrail bolted on afterward.

## Architecture

The agent core (`internal/ai_model`, `internal/agent`, `internal/tools`) is provider- and interface-independent — the same loop runs behind a terminal (`cmd/vohu`) or an HTTP server (`cmd/server`) with no changes to the loop itself.

| Package | Responsibility |
|---|---|
| [`internal/ai_model`](internal/ai_model) | Provider-agnostic types: the `LLM` interface, `ChatRequest`/`ChatResponse`, `Message`, `Role` (`user`/`assistant`/`tool`), `ToolCall`/`ToolResult`. |
| [`internal/ai_model/models`](internal/ai_model/models) | Provider implementations of `LLM` — Gemini (`google.golang.org/genai`), Anthropic (`anthropic-sdk-go`), and any OpenAI-compatible endpoint (OpenAI itself, or a custom base URL — DeepSeek, Groq, a local Ollama server, ...) — each translating the generic conversation history into its own wire format. |
| [`internal/agent`](internal/agent) | The agent loop itself. Calls the model, executes any requested tools through the registry, feeds the results back, and repeats until the model responds without further tool calls. Has no knowledge of terminals, providers, or specific tools. |
| [`internal/tools`](internal/tools) | The `Tool` interface and the `Registry`: register tools, look them up by name, and auto-derive the tool definitions sent to the model — no manually duplicated list to keep in sync. |
| [`internal/tools/system_tools`](internal/tools/system_tools) | Example tool: current system time. |
| [`internal/tools/command`](internal/tools/command) | Command execution: `Command`, a rule-based `Policy` (accept-mode allow-list or prohibited-mode deny-list, matched on program + argument prefixes), a `LocalExecutor` and an `SSHExecutor` that evaluate the policy before anything runs, and `TestDial` (verifies a private key actually authenticates before a connection is ever saved). |
| [`cmd/vohu`](cmd/vohu) | Terminal entry point — prompts for a provider/model, then runs the chat loop. |
| [`cmd/server`](cmd/server) | HTTP entry point — the same agent loop behind a multi-user platform (see below). |
| [`internal/platform`](internal/platform) | Everything the platform (users, auth, resource access, SSH connections, conversations, chat) is built from — see the section below. |
| [`web`](web) | React + TypeScript + Tailwind + shadcn/ui admin frontend for `cmd/server` (English + Persian/RTL). |

Adding a new tool means implementing the three-method `Tool` interface and registering it — the agent loop and the model-facing tool definitions update automatically. Adding a new model provider means implementing `LLM.Chat` — nothing else in the codebase changes.

## The platform (`cmd/server` + `web/`)

A multi-user deployment of the agent loop: users log in, hold SSH connections to remote hosts, and chat with the agent — which can use those connections as tools — through a browser.

| Module | Responsibility |
|---|---|
| [`internal/platform/user`](internal/platform/user) | User accounts — CRUD, bcrypt password hashing. |
| [`internal/platform/auth`](internal/platform/auth) | Login, JWT access/refresh tokens, the authentication middleware every other module's routes sit behind. |
| [`internal/platform/rbac`](internal/platform/rbac) | The platform's one authorization mechanism: `ResourceAccess` rows grant a level (`read`/`write`/`manage`) on a resource type + ID to a grantee — a specific user, or every member of a role at once. A grant on a role cascades to its members; a more specific per-user row (including an explicit `prohibited` one) always wins over a role-level grant. No flat permission-key system alongside it — this is the only mechanism. |
| [`internal/platform/sshconn`](internal/platform/sshconn) | SSH connections users can grant the agent access to — private-key auth only. A connection is test-dialed (real handshake, no command run) before it's ever saved, so a bad host or mismatched key is caught immediately rather than the first time the agent tries to use it. The key is AES-GCM encrypted at rest and never returned by the API. |
| [`internal/platform/conversation`](internal/platform/conversation) | Chat threads and their messages. Two separate read paths: `LoadHistory` (unpaginated, feeds the agent's own context) and `ListMessages` (paginated, newest page first, for the chat UI's scroll-up-to-load-older behavior). |
| [`internal/platform/providerkey`](internal/platform/providerkey) | One API key per provider, per user or global (personal always wins) — the account-level default for Gemini/Anthropic/OpenAI. |
| [`internal/platform/custommodel`](internal/platform/custommodel) | Any number of *named* OpenAI-compatible presets (URL + key + model), per user or global — unlike `providerkey`'s single slot, this is what lets one account use a local Ollama server *and* a DeepSeek account at once. |
| [`internal/platform/chat`](internal/platform/chat) | Wires a conversation's provider/model (or custom preset) into a concrete `ai_model.LLM`, runs the agent loop with the caller's SSH connections registered as tools, and streams the reply back over SSE. |
| [`internal/platform/httpserver`](internal/platform/httpserver), [`shared`](internal/platform/shared), [`migrations`](internal/platform/migrations), [`seeders`](internal/platform/seeders) | Gin router setup, the generic CRUD/pagination/filtering helpers every module builds on, and schema migration + default-data seeding on startup. |

## Getting started

### Agent core only (terminal, no database)

```bash
git clone https://github.com/Aliizi83/vohu.git
cd vohu
export GEMINI_API_KEY=...      # to use Gemini
export ANTHROPIC_API_KEY=...   # to use Claude
go run ./cmd/vohu
```

You'll be prompted to pick a model before the chat starts; only the API key for the provider you pick needs to be set.

### Full platform (server + web app)

Needs a Postgres database — see `config/config-development.yml` for the expected connection settings (defaults to `localhost:5432`, db `vohu_db`, user/password `postgres`/`postgres`; override via `APP_ENV` + a matching `config-<env>.yml`, or start a local Postgres container matching those defaults).

```bash
go run ./cmd/server      # migrates the schema and seeds default data on first run
cd web && npm install && npm run dev
```

Log in with the seeded default admin — `admin` / `change-me-now` — and change the password before this is ever exposed beyond localhost. The dev-only JWT secrets and AES encryption key in `config-development.yml` need replacing too before any real deployment.

## Tools available today

- `get_current_system_time` — returns the current system time.
- `execute_command` — runs a program with arguments after checking it against the security policy (e.g. `git status`/`git log` allowed, everything else on `git` denied by default).
- `ssh_execute` / `list_ssh_connections` *(platform only)* — same policy-gated execution, over SSH against a connection the caller holds resource access to.

## Supported models

- Gemini Flash, Gemini Flash Lite 3.5
- Claude Opus 5, Claude Sonnet 5, Claude Haiku 4.5
- Any OpenAI-compatible endpoint — OpenAI itself, or a custom base URL (DeepSeek, Groq, a local Ollama server, ...)

## Roadmap

- [x] Agent loop
- [x] Tool registry with auto-derived tool definitions
- [x] Command execution tool with a security policy
- [x] Multi-provider support (Gemini, Anthropic, OpenAI-compatible)
- [x] HTTP/API mode — the same agent loop behind a server instead of a terminal
- [x] Per-session conversation persistence, with paginated history
- [x] SSH tool, with per-connection resource access and pre-save connection testing
- [ ] Structured parameter schemas for tool definitions
- [ ] Additional tools (Docker, file I/O)
- [ ] Broader test coverage

## License

MIT
