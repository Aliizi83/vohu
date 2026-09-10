<img src="web/public/favicon.svg" alt="Vohu logo" width="72" height="72">

# Vohu

[![Go](https://github.com/Aliizi83/vohu/actions/workflows/go.yml/badge.svg)](https://github.com/Aliizi83/vohu/actions/workflows/go.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**A provider-independent AI agent runtime for Go.** Vohu runs a real tool-calling agent loop — not a one-shot chat wrapper — behind either a terminal or a full multi-user web platform, sharing the exact same core.

> From thought to action.

## What Vohu actually does

```
user message → model → tool call → tool execution → tool result → model → ... → final response
```

The model isn't just generating text — it can ask to run a shell command or reach a remote server over SSH, see the real output, and decide what to do next, looping until it has an actual answer instead of a guess. Every tool call passes through an explicit, evaluated-before-execution security policy first; nothing runs just because the model asked.

This repository is two things built on that same loop:

- **`cmd/vohu`** — a terminal chat client. Pick a model, start talking. No database, no setup beyond an API key.
- **`cmd/server` + `web/`** — a multi-user platform. Users log in, register SSH connections and API keys, and chat with an agent that can use those connections as tools, through a browser.

Both run the identical `internal/agent` loop against the identical `internal/tools` — the platform doesn't fork the core to add multi-tenancy, it wraps it.

## Quickstart

### Terminal (fastest way to see it work)

```bash
git clone https://github.com/Aliizi83/vohu.git
cd vohu
export GEMINI_API_KEY=...      # or ANTHROPIC_API_KEY, or nothing (pick "OpenAI-compatible" and enter a key at the prompt)
go run ./cmd/vohu
```

You'll be prompted to pick a model, then you're chatting — with the full tool set below available out of the box, rooted at the directory you ran it from.

### Platform (server + web app)

```bash
# 1. Postgres + pgadmin, matching config/config-development.yml's defaults
cp docker/.env.example docker/.env
docker compose -f docker/docker-compose.yml --env-file docker/.env up -d

# 2. API server — migrates the schema and seeds default data on first run
go run ./cmd/server

# 3. Frontend, in a second terminal
cd web && npm install && npm run dev
```

Log in at the printed dev URL with the seeded default admin: **`admin` / `change-me-now`**. Change that password — and the dev-only JWT secrets and AES key in `config/config-development.yml` — before this is ever exposed beyond localhost.

## How it's organized

The agent core knows nothing about HTTP, databases, or multiple users — that separation is deliberate, not incidental.

| Package | Responsibility |
|---|---|
| [`internal/ai_model`](internal/ai_model) | Provider-agnostic types: the `LLM` interface, `ChatRequest`/`ChatResponse`, `Message`, `ToolCall`/`ToolResult`. |
| [`internal/ai_model/models`](internal/ai_model/models) | `LLM` implementations — Gemini ([`google.golang.org/genai`](https://pkg.go.dev/google.golang.org/genai)), Anthropic ([`anthropic-sdk-go`](https://github.com/anthropics/anthropic-sdk-go)), and OpenAI/OpenAI-compatible ([`openai-go`](https://github.com/openai/openai-go), any base URL — DeepSeek, Groq, a local Ollama server, ...). |
| [`internal/agent`](internal/agent) | The loop itself: calls the model, executes requested tools through the registry, feeds results back, repeats until the model stops asking for tools. No knowledge of terminals, providers, or specific tools. |
| [`internal/tools`](internal/tools) | The `Tool` interface and `Registry` — register a tool, and its model-facing definition is auto-derived. Nothing to keep in sync by hand. |
| [`internal/tools/command`](internal/tools/command) | `Command`, a rule-based `Policy` (allow-list or deny-list, matched on program + argument prefixes), a `LocalExecutor`/`SSHExecutor` that check the policy before anything runs, `execute_command` (one program, no shell) and `execute_shell` (`sh -c`, so pipes/redirects work — policy is checked against shell execution as a whole, since there's no sound way to allow-list what's chained inside an arbitrary shell string), and `TestDial` for verifying an SSH key works before a connection is ever saved. |
| [`internal/tools/filesystem`](internal/tools/filesystem) | `read_file`, `write_file` (refuses to overwrite a file that hasn't been read first), `edit_file` (exact-match str_replace), `list_directory`, `search_files` (grep), `find_files` (glob, `**` supported) — every one resolved through a shared `Workspace` that rejects any path escaping its root, string-based or via a symlink. |
| [`internal/tools/network`](internal/tools/network) | `http_fetch` — HTML responses come back as extracted text, not raw markup. |
| [`internal/tools/system_tools`](internal/tools/system_tools) | Example tool: current system time. |
| [`cmd/vohu`](cmd/vohu) | Terminal entry point. |
| [`cmd/server`](cmd/server) | HTTP entry point — composition root for every `internal/platform` module below. |

## The platform

| Module | Responsibility |
|---|---|
| [`internal/platform/user`](internal/platform/user) | Accounts — CRUD, bcrypt password hashing. |
| [`internal/platform/auth`](internal/platform/auth) | Login, JWT access/refresh tokens, the middleware every other module's routes sit behind. |
| [`internal/platform/rbac`](internal/platform/rbac) | The one authorization mechanism in the platform. A `ResourceAccess` row grants a level (`read`/`write`/`manage`) on a resource type + ID to a grantee — a specific user, or every member of a role at once. A role-level grant cascades to its members; a more specific per-user row (including an explicit `prohibited` one) always wins. No separate flat-permission system running alongside it. |
| [`internal/platform/sshconn`](internal/platform/sshconn) | SSH connections a user can grant the agent access to. Private-key auth only; a connection is test-dialed (real handshake, no command run) *before* it's saved, so a bad host or mismatched key fails loudly at creation time, not mid-conversation. Keys are AES-GCM encrypted at rest and never returned by the API. |
| [`internal/platform/conversation`](internal/platform/conversation) | Chat threads and messages, with two separate read paths on purpose: `LoadHistory` (unpaginated, feeds the agent's own context) and `ListMessages` (paginated, newest page first, for the UI's scroll-up-to-load-older behavior). |
| [`internal/platform/providerkey`](internal/platform/providerkey) | One API key per provider, per user or global — a personal key always wins over the account-wide default. |
| [`internal/platform/custommodel`](internal/platform/custommodel) | Any number of *named* OpenAI-compatible presets (URL + key + model), per user or global — where `providerkey` gives one "openai" slot, this is what lets one account use a local Ollama server *and* a DeepSeek account side by side. |
| [`internal/platform/chat`](internal/platform/chat) | Resolves a conversation's provider/model (or custom preset) into a concrete `ai_model.LLM`, runs the agent loop with the caller's SSH connections registered as tools, streams the reply back over SSE. |
| [`internal/platform/shared`](internal/platform/shared) | Generic CRUD/pagination/dynamic-filter helpers every module builds on, so list endpoints, response envelopes, and access-level route guards aren't reimplemented per module. |
| [`internal/platform/httpserver`](internal/platform/httpserver) / [`migrations`](internal/platform/migrations) / [`seeders`](internal/platform/seeders) | Gin router setup; schema migration + default-data seeding on startup. |
| [`web`](web) | React + TypeScript + Tailwind + shadcn/ui frontend — English and Persian, full RTL support. |

## Safe by default

Two independent mechanisms, not one blanket guardrail:

- **What a tool is allowed to do** — every command (local or over SSH) is checked against a `command.Policy` *before* it runs: an accept-mode allow-list or a prohibited-mode deny-list, matched on the program name and argument prefixes. The model can ask for anything; only what the policy permits actually executes.
- **What a user is allowed to see** — in the platform, every resource (an SSH connection, a conversation, another user) is gated by `rbac.ResourceAccess`. A caller with no grant on a conversation gets a 404, not a 403 — existence itself isn't leaked to someone with no access.

## API docs

Every `cmd/server` endpoint (all 42 of them) is documented with Swagger/OpenAPI — generated from `@Summary`/`@Param`/`@Success`/... comments on each handler via [swaggo/swag](https://github.com/swaggo/swag). With the server running, open `/swagger/index.html` for the interactive UI (`/swagger/doc.json` for the raw spec).

A handler's annotations changing means regenerating `docs/` (committed, since `cmd/server` imports it — the build doesn't call `swag` itself):

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
```

## Development

```bash
go build ./... && go vet ./... && gofmt -l .   # build, vet, format check
go test ./...                                   # unit + integration tests (sqlite in-memory, no DB needed)

cd web
npm run build    # tsc -b && vite build
npm run lint     # oxlint
```

## Supported models

- Gemini Flash, Gemini Flash Lite 3.5
- Claude Opus 5, Claude Sonnet 5, Claude Haiku 4.5
- Any OpenAI-compatible endpoint — OpenAI itself, or a custom base URL (DeepSeek, Groq, a local Ollama server, ...)

## Roadmap

- [x] Agent loop with tool-calling and a security policy
- [x] Multi-provider support (Gemini, Anthropic, OpenAI-compatible)
- [x] HTTP platform: users, roles, polymorphic resource access
- [x] SSH connections as agent tools, with pre-save connection testing
- [x] Persisted, paginated conversations
- [x] Web frontend (React, i18n, RTL)
- [x] Filesystem tools (read/write/edit/list/search/find), scoped to a workspace root
- [x] Shell + HTTP fetch tools
- [ ] Web search
- [ ] Structured parameter schemas for tool definitions
- [ ] Additional tools (Docker, a persistent task list)
- [ ] Broader test coverage

## License

MIT — see [LICENSE](LICENSE).
