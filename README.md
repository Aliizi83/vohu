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

| Package | Responsibility |
|---|---|
| [`internal/ai_model`](internal/ai_model) | Provider-agnostic types: the `LLM` interface, `ChatRequest`/`ChatResponse`, `Message`, `Role` (`user`/`assistant`/`tool`), `ToolCall`/`ToolResult`. |
| [`internal/ai_model/models`](internal/ai_model/models) | Provider implementations of `LLM` — Gemini (`google.golang.org/genai`) and Anthropic (`anthropic-sdk-go`) — each translating the generic conversation history into its own wire format. |
| [`internal/agent`](internal/agent) | The agent loop itself. Calls the model, executes any requested tools through the registry, feeds the results back, and repeats until the model responds without further tool calls. Has no knowledge of terminals, providers, or specific tools. |
| [`internal/tools`](internal/tools) | The `Tool` interface and the `Registry`: register tools, look them up by name, and auto-derive the tool definitions sent to the model — no manually duplicated list to keep in sync. |
| [`internal/tools/system_tools`](internal/tools/system_tools) | Example tool: current system time. |
| [`internal/tools/command`](internal/tools/command) | Command execution: `Command`, a rule-based `Policy` (accept-mode allow-list or prohibited-mode deny-list, matched on program + argument prefixes), and a `LocalExecutor` that evaluates the policy before anything runs. |
| [`cmd/vohu`](cmd/vohu) | Terminal entry point — prompts for a provider/model, then runs the chat loop. |

Adding a new tool means implementing the three-method `Tool` interface and registering it — the agent loop and the model-facing tool definitions update automatically. Adding a new model provider means implementing `LLM.Chat` — nothing else in the codebase changes.

## Getting started

```bash
git clone https://github.com/Aliizi83/vohu.git
cd vohu
export GEMINI_API_KEY=...      # to use Gemini
export ANTHROPIC_API_KEY=...   # to use Claude
go run ./cmd/vohu
```

You'll be prompted to pick a model before the chat starts; only the API key for the provider you pick needs to be set.

## Tools available today

- `get_current_system_time` — returns the current system time.
- `execute_command` — runs a program with arguments after checking it against the security policy (e.g. `git status`/`git log` allowed, everything else on `git` denied by default).

## Supported models

- Gemini Flash, Gemini Flash Lite 3.5
- Claude Opus 5, Claude Sonnet 5, Claude Haiku 4.5

## Roadmap

- [x] Agent loop
- [x] Tool registry with auto-derived tool definitions
- [x] Command execution tool with a security policy
- [x] Multi-provider support (Gemini, Anthropic)
- [ ] HTTP/API mode — the same agent loop behind a server instead of a terminal
- [ ] Per-session conversation persistence
- [ ] Structured parameter schemas for tool definitions
- [ ] Additional tools (SSH, Docker, file I/O)
- [ ] Test coverage

## License

MIT
