# Vohu

**A self-hosted AI agent runtime for developers and infrastructure.**

Vohu is a Go-based AI agent platform designed to give LLMs controlled access to real development environments, local systems, and remote infrastructure through structured tools and explicit execution policies.

Instead of building another AI chat application, Vohu focuses on the runtime behind an agent:

* Provider-independent LLM integration
* Structured tool calling
* Local filesystem operations
* Command execution with security policies
* SSH-based remote execution
* Conversation persistence
* Role-based access control
* CLI and web interfaces sharing the same agent core

The goal is simple:

> **Let AI interact with real systems without giving it unrestricted control.**

---

## ✨ Features

### 🤖 Agent Runtime

Vohu implements a tool-using agent loop:

```text
User
  │
  ▼
Agent
  │
  ▼
LLM
  │
  ├── Final response ──────────────► User
  │
  └── Tool call
          │
          ▼
       Tool Registry
          │
          ▼
       Tool execution
          │
          ▼
       Tool result
          │
          └──────────────► LLM
```

The agent can execute multiple tool calls across several iterations before producing a final response.

The core agent is intentionally independent of HTTP, UI, persistence, and individual tools, allowing the same runtime to power different interfaces.

---

## 🧠 Multi-Provider LLM Architecture

Vohu uses a provider abstraction so the agent does not depend on a specific LLM vendor.

Currently supported integrations include:

* Google Gemini
* OpenAI
* OpenAI-compatible APIs
* Anthropic

The internal agent works with provider-independent concepts such as:

* Messages
* Tool definitions
* Tool calls
* Tool results
* Provider metadata

Provider-specific details are handled inside their respective adapters.

```text
                  Agent Core
                      │
                      ▼
                 LLM Interface
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
       Gemini       OpenAI    Anthropic
          │           │           │
          ▼           ▼           ▼
       Provider-specific API
```

This allows the same tools and agent logic to work across different model providers.

---

# 🛠️ Tool System

Tools are first-class components in Vohu.

A tool provides:

* A name
* A description
* Structured parameters
* An execution method
* A structured result

Tools are registered in a central registry and their definitions can be exposed directly to the LLM.

This means adding a new capability does not require modifying the agent loop itself.

```text
Tool
 ├── Name
 ├── Description
 ├── Parameters
 └── Execute()
       │
       ▼
   ToolResult
```

### Current tool categories

#### Filesystem

* Read files
* Write files
* Edit files
* List directories
* Search files
* Find files

Filesystem tools operate inside a configured workspace and include protections against escaping the workspace through paths or symlinks.

Large outputs are bounded to avoid unnecessarily filling the model context.

---

### Command Execution

Vohu provides command execution tools with explicit execution policies.

Policies can control commands based on:

* Program
* Argument prefixes
* Allow/deny behavior

For example:

```text
git status
git log
docker ps
docker logs
```

can be explicitly permitted while unknown commands can be denied.

Two policy modes are available:

* **Accept mode** — commands must be explicitly allowed.
* **Prohibited mode** — explicitly prohibited commands are denied.

This allows deployments to choose between restrictive and permissive execution models.

---

### Shell Execution

Vohu also provides shell execution for cases where a command needs shell semantics.

Because shell commands can contain arbitrary command chains and shell syntax, shell execution should be treated as a higher-risk capability than structured command execution.

---

### SSH

Vohu can execute commands on remote systems through SSH.

The same command policy concepts can be applied to remote execution, allowing an agent to interact with infrastructure without giving it unrestricted access.

```text
Agent
  │
  ▼
SSH Tool
  │
  ▼
Policy
  │
  ▼
Remote Host
```

---

### System Tools

Vohu also includes system-level tools such as retrieving the current system time.

The tool architecture is intentionally extensible so additional capabilities can be introduced without changing the agent core.

---

# 🔐 Security

Vohu is designed around the assumption that an LLM should **not automatically receive unrestricted access to the system**.

Several layers are used to constrain agent capabilities.

### Workspace Isolation

Filesystem operations are resolved against a configured workspace.

Path validation includes protection against:

* `..` traversal
* Absolute paths escaping the workspace
* Symlink-based workspace escapes

---

### Command Policies

Command execution is evaluated by a policy layer before reaching the operating system.

```text
LLM
 │
 ▼
Tool
 │
 ▼
Command Policy
 │
 ├── Allowed ───────► Executor
 │
 └── Denied ────────► Tool Error
```

This keeps execution policy separate from the LLM and the command executor.

---

### Safer File Modification

File-writing operations include additional safeguards.

For example, modifying an existing file requires the file to have been read by the agent first.

The `edit_file` tool also requires an exact match for the target text, preventing ambiguous replacements when the same content appears multiple times.

---

# 🧩 Context-Aware Tooling

Vohu's tools are designed with LLM context limits in mind.

For example:

* File reads are bounded.
* File reads support offsets and limits.
* Directory listings have entry limits.
* Common high-volume directories can be excluded from recursive listings.
* Tool results are returned through structured result objects.

These constraints prevent a single tool call from accidentally flooding the model context with an entire repository, log file, or generated directory.

---

# 👥 Conversations & Access Control

Vohu includes persistent conversations and user-oriented access control.

The platform supports:

* Users
* Roles
* Resource access
* Explicit permissions
* Explicit prohibitions

Permission decisions can distinguish between operations such as:

```text
read
write
manage
prohibited
```

Explicit prohibitions can override broader grants, allowing more restrictive user-specific policies.

---

# 🌐 Interfaces

The same agent core can be used through multiple interfaces.

```text
                    Agent Core
                   /          \
                  /            \
                 ▼              ▼
              CLI/TUI         Web/API
```

The CLI and web application do not implement separate agent logic.

This keeps the behavior of the agent consistent regardless of how it is accessed.

---

## Web Architecture

The web layer provides the application-facing API and streaming communication while delegating agent execution to the same core runtime.

The frontend is built with React and TypeScript.

Streaming responses allow the UI to display model output and agent activity without waiting for the entire response to finish.

---

# 🏗️ Architecture

At a high level, Vohu is organized around several independent layers:

```text
┌──────────────────────────────────────────────┐
│                 Interfaces                   │
│                                              │
│              CLI / Web / API                 │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                 Agent Core                   │
│                                              │
│       Agent Loop / Context / Tool Calls      │
└───────────────┬──────────────────┬───────────┘
                │                  │
                ▼                  ▼
        ┌──────────────┐    ┌──────────────┐
        │ LLM Provider │    │ Tool Registry│
        └──────┬───────┘    └──────┬───────┘
               │                   │
        ┌──────┼──────┐      ┌─────┼─────────────┐
        ▼      ▼      ▼      ▼     ▼      ▼      ▼
     Gemini  OpenAI  Claude  FS  Command  SSH  System
```

The important architectural boundary is between the **agent runtime**, **providers**, and **tools**.

The agent should not need to know how a particular model provider implements tool calling, nor how a particular tool executes its operation.

---

# 📁 Project Structure

A simplified view of the repository:

```text
vohu/
├── cmd/
│   ├── vohu/          # CLI application
│   └── server/        # Web/API server
│
├── internal/
│   ├── agent/         # Agent runtime and execution loop
│   │
│   ├── ai_model/      # LLM abstraction and providers
│   │   └── models/
│   │       ├── gemini/
│   │       ├── openai/
│   │       └── anthropic/
│   │
│   ├── tools/         # Agent tools
│   │   ├── filesystem/
│   │   ├── command/
│   │   ├── ssh/
│   │   └── ...
│   │
│   ├── platform/      # Application/platform services
│   │
│   └── ...
│
└── ...
```

The exact package structure may evolve as the project grows, but the core separation remains:

**Agent → Provider / Tools → Infrastructure**

---

# 🚀 Getting Started

## Requirements

* Go
* A supported LLM provider/API key
* Node.js for the web frontend
* Optional: SSH access for remote execution

---

## Run the CLI

Clone the repository:

```bash
git clone https://github.com/Aliizi83/vohu.git
cd vohu
```

Configure your model provider credentials and run:

```bash
go run ./cmd/vohu
```

---

## Run the Server

Start the backend:

```bash
go run ./cmd/server
```

Then start the frontend according to the frontend project configuration.

---

# ⚙️ Configuration

Vohu is designed to keep provider configuration separate from the agent runtime.

Typical configuration includes:

```text
LLM provider
API key
Model
Workspace
Tool policies
SSH connections
Application settings
```

Provider-specific configuration is handled by the corresponding adapter while the agent itself remains provider-agnostic.

---

# 🔌 Extending Vohu

Adding a new tool should not require changing the agent loop.

A typical tool follows this conceptual structure:

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() ToolParameters
    Execute(ctx context.Context, args map[string]any) (ToolResult, error)
}
```

Once registered, the tool can become available to the agent and its structured definition can be exposed to compatible LLM providers.

This makes Vohu suitable for gradually adding capabilities such as:

* Git
* Docker
* Kubernetes
* Databases
* Cloud infrastructure
* Monitoring systems
* CI/CD systems
* Developer workflows

without coupling those capabilities to the core agent implementation.

---

# 🎯 Design Goals

Vohu is built around several principles.

### 1. Provider Independence

The agent should not be tightly coupled to one model vendor.

### 2. Controlled Capabilities

LLMs should interact with systems through explicit tools rather than unrestricted access.

### 3. Secure Defaults

Filesystem and command execution should have meaningful boundaries.

### 4. Small Agent Core

The agent loop should remain simple even as the number of tools and providers grows.

### 5. Real System Interaction

Vohu is intended to interact with real development environments and infrastructure, not just generate text.

### 6. Self Hosting

The platform is designed to run under the user's own infrastructure and configuration.

---

# 🧪 Current Status

Vohu is actively evolving.

The current implementation already provides the core pieces required for a practical tool-using AI agent:

* Multi-provider LLM support
* Agent execution loop
* Structured tool calling
* Filesystem operations
* Command execution
* Command policies
* SSH execution
* Conversation persistence
* Access control
* CLI
* Web/API
* Streaming responses

The project is currently focused on strengthening the runtime, safety model, context management, and infrastructure capabilities rather than simply adding more model integrations.

---

# 🗺️ Roadmap

Potential areas of development include:

* [ ] Parallel tool execution
* [ ] Better context management and compaction
* [ ] Dynamic tool discovery
* [ ] Improved tool error semantics
* [ ] Approval workflows for sensitive operations
* [ ] Git tools
* [ ] Docker tools
* [ ] Kubernetes tools
* [ ] Background tasks
* [ ] Scheduled agents
* [ ] Agent observability and tracing
* [ ] More infrastructure integrations
* [ ] MCP integration

The roadmap is intentionally focused on improving the agent runtime rather than turning Vohu into a collection of unrelated features.

---

# 🤝 Contributing

Contributions, ideas, and discussions are welcome.

If you want to add a new capability, prefer implementing it as an independent tool or provider adapter rather than modifying the agent core.

For larger architectural changes, opening an issue or discussion first is recommended.

---

# 📄 License

See the repository license for details.

---

## Why Vohu?

Most AI applications focus on the interface between the user and the model.

Vohu focuses on what happens **after the model decides to act**.

The interesting problem is not only:

> "How do I ask an LLM a question?"

It is:

> "How can an LLM safely interact with the systems that matter?"

Vohu explores that problem through a small, provider-independent agent runtime built in Go.