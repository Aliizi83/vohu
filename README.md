# Vohu

<p align="center">
  <strong>A self-hosted AI agent runtime for developers and infrastructure.</strong>
</p>

<p align="center">
  Give AI access to real systems — with tools, policies, boundaries, and control.
</p>

<p align="center">
  <a href="https://github.com/Aliizi83/vohu">GitHub</a>
  ·
  <a href="#-quick-start">Quick Start</a>
  ·
  <a href="#-architecture">Architecture</a>
  ·
  <a href="#-tools">Tools</a>
  ·
  <a href="#-roadmap">Roadmap</a>
</p>

---

## 🧠 What is Vohu?

Vohu is a **self-hosted AI agent runtime written in Go**.

It allows LLMs to interact with real development environments and infrastructure through structured tools instead of unrestricted system access.

An agent can:

* Read and modify files
* Search through a project
* Execute commands
* Interact with remote machines over SSH
* Inspect system state
* Work across multiple tool calls
* Maintain conversations
* Operate through a CLI or web interface

The important part is not simply connecting an LLM to a terminal.

Vohu provides the runtime around that interaction:

```text
                        ┌──────────────┐
                        │     User     │
                        └──────┬───────┘
                               │
                               ▼
                        ┌──────────────┐
                        │     Vohu     │
                        │ Agent Runtime│
                        └──────┬───────┘
                               │
                         Tool Calling
                               │
             ┌─────────────────┼─────────────────┐
             │                 │                 │
             ▼                 ▼                 ▼
        Filesystem          Commands            SSH
             │                 │                 │
             └─────────────────┼─────────────────┘
                               │
                               ▼
                        Real Environment
```

> **Vohu is built around a simple idea: AI should be able to act, but its capabilities should be explicit and controllable.**

---

# ✨ Highlights

<table>
<tr>
<td width="50%">

### 🤖 Agent Runtime

A real tool-using agent loop with iterative tool execution and model feedback.

</td>
<td width="50%">

### 🔌 Multi-Provider

Gemini, OpenAI, OpenAI-compatible APIs, and Anthropic through a provider abstraction.

</td>
</tr>

<tr>
<td>

### 🛠️ Structured Tools

Tools expose names, descriptions, parameters, and structured results to LLMs.

</td>
<td>

### 🔐 Execution Policies

Commands can be controlled using explicit allow/deny policies before execution.

</td>
</tr>

<tr>
<td>

### 📁 Workspace Isolation

Filesystem operations are restricted to a configured workspace, including symlink-aware path validation.

</td>
<td>

### 🌐 Local + Remote

Interact with local systems and remote infrastructure through SSH.

</td>
</tr>

<tr>
<td>

### 💬 Persistent Conversations

Conversations are persisted and can be reused as agent context.

</td>
<td>

### 🖥️ CLI + Web

Different interfaces share the same underlying agent runtime.

</td>
</tr>
</table>

---

# 🎬 How It Works

Vohu follows an iterative agent loop:

```text
┌──────────┐
│   User   │
└────┬─────┘
     │
     ▼
┌──────────┐
│   Agent  │
└────┬─────┘
     │
     ▼
┌──────────┐
│   LLM    │
└────┬─────┘
     │
     ├───────────────┐
     │               │
     ▼               ▼
Final Answer      Tool Call
                     │
                     ▼
               ┌──────────┐
               │   Tool   │
               └────┬─────┘
                    │
                    ▼
               Tool Result
                    │
                    └──────────────► LLM
```

For example:

```text
User:
"Why is my nginx service failing?"

        ↓

LLM
        ↓
ssh_execute("systemctl status nginx")

        ↓

Tool Result
        ↓
LLM
        ↓
ssh_execute("journalctl -u nginx --no-pager -n 50")

        ↓

Tool Result
        ↓

LLM
        ↓

"nginx is failing because..."
```

The agent decides **which tool to use and when**, while Vohu controls how that tool is executed.

---

# 🧩 Tools

Tools are first-class components in Vohu.

Each tool provides structured information that can be translated into the native tool/function-calling format of supported model providers.

Conceptually:

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

This keeps tools independent from the LLM provider.

## Current capabilities

### 📁 Filesystem

* `read_file`
* `write_file`
* `edit_file`
* `list_directory`
* `search_files`
* `find_files`

Filesystem tools include:

* Workspace boundaries
* Path traversal protection
* Symlink-aware validation
* Bounded file reads
* Offset/limit support
* Directory entry limits
* Large-directory exclusions

---

### ⚙️ Command Execution

Vohu can execute system commands through a policy-controlled execution layer.

Commands can be restricted using:

```text
Program
Arguments / prefixes
Allow / deny decision
```

For example:

```text
git status
git log
docker ps
docker logs
```

can be explicitly allowed while unknown commands can remain blocked.

Two policy strategies are available:

```text
Accept Mode
───────────
Unknown command → Deny


Prohibited Mode
───────────────
Unknown command → Allow
Explicitly prohibited → Deny
```

---

### 🐚 Shell Execution

For commands requiring shell semantics, Vohu also provides shell execution.

Because shell commands can contain pipelines, command chains, substitutions, and other shell features, shell execution represents a higher-trust capability than structured command execution.

---

### 🌐 SSH

Vohu can interact with remote machines through SSH.

The same execution-policy concepts can be applied to remote commands.

```text
                  Vohu
                    │
                    ▼
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

This makes infrastructure-oriented workflows possible without embedding SSH logic into the agent itself.

---

### 🕐 System

System-level capabilities such as retrieving the current system time are exposed through the same tool architecture.

---

# 🔌 LLM Providers

Vohu separates the agent runtime from provider-specific APIs.

```text
                       Agent
                         │
                         ▼
                    LLM Interface
                         │
          ┌──────────────┼──────────────┐
          │              │              │
          ▼              ▼              ▼
       Gemini          OpenAI       Anthropic
          │              │              │
          ▼              ▼              ▼
       Provider-specific adapters
```

The agent works with internal concepts such as:

* Messages
* Tool definitions
* Tool calls
* Tool results
* Provider metadata

Provider-specific behavior stays inside the adapters.

### Supported

| Provider               | Tool Calling | Notes                   |
| ---------------------- | ------------ | ----------------------- |
| Google Gemini          | ✅            | Native function calling |
| OpenAI                 | ✅            | Native tool calling     |
| OpenAI-compatible APIs | ✅            | Configurable base URL   |
| Anthropic              | ✅            | Native tool use         |

---

# 🔐 Security Model

Vohu treats LLM access to the operating system as a capability that should be explicitly controlled.

## Filesystem boundaries

Filesystem tools operate inside a configured workspace.

Path resolution accounts for:

* Relative path traversal
* Absolute paths
* Existing symlinks
* Workspace containment

A symlink such as:

```text
workspace/link → /etc
```

should not allow:

```text
read_file("link/passwd")
```

to escape the workspace.

---

## Command policies

The command executor is separated from the policy engine:

```text
LLM
 │
 ▼
Tool
 │
 ▼
Policy
 │
 ├── Allowed ───────► Executor ─────► OS
 │
 └── Denied ────────► Tool Result
```

This means the model does not directly control the executor.

---

## Safer file modification

Vohu includes additional safeguards around file modification.

For existing files, the agent is expected to have read the file before overwriting it.

`edit_file` also requires an exact target match, avoiding ambiguous replacements when the same text occurs multiple times.

---

# 🏗️ Architecture

Vohu is intentionally divided into independent layers.

```text
┌───────────────────────────────────────────────────┐
│                   Interfaces                      │
│                                                   │
│                   CLI / Web                       │
└────────────────────────┬──────────────────────────┘
                         │
                         ▼
┌───────────────────────────────────────────────────┐
│                   Agent Core                      │
│                                                   │
│            Agent Loop / Tool Calls                │
└───────────────┬───────────────────┬───────────────┘
                │                   │
                ▼                   ▼
        ┌───────────────┐   ┌────────────────┐
        │ LLM Providers │   │ Tool Registry  │
        └───────┬───────┘   └───────┬────────┘
                │                   │
        ┌───────┼────────┐    ┌─────┼─────────────┐
        ▼       ▼        ▼    ▼     ▼      ▼      ▼
     Gemini   OpenAI  Claude FS  Command  SSH   System
```

The core principle is:

> **The agent should not know how a provider works, and the provider should not know how a tool works.**

---

# 📦 Project Structure

```text
vohu/
│
├── cmd/
│   ├── vohu/              # CLI application
│   └── server/            # Web/API server
│
├── internal/
│   │
│   ├── agent/             # Agent runtime
│   │
│   ├── ai_model/          # LLM abstraction
│   │   └── models/        # Provider adapters
│   │
│   ├── tools/             # Agent capabilities
│   │   ├── filesystem/
│   │   ├── command/
│   │   ├── ssh/
│   │   └── ...
│   │
│   └── platform/          # Application/platform services
│
└── ...
```

The goal is to keep the core runtime independent from the interface through which it is used.

---

# 💻 CLI + Web

Vohu can expose the same agent through different interfaces.

```text
                   Agent Core
                  /          \
                 /            \
                ▼              ▼
             CLI/TUI        Web/API
```

This avoids maintaining separate implementations of the agent for different clients.

The web interface uses streaming communication so model output can be delivered progressively to the frontend.

---

# 🚀 Quick Start

## Requirements

* Go
* API key for a supported LLM provider
* Node.js for the web frontend
* Optional SSH access for remote execution

### Clone

```bash
git clone https://github.com/Aliizi83/vohu.git
cd vohu
```

### Run the CLI

```bash
go run ./cmd/vohu
```

### Run the server

```bash
go run ./cmd/server
```

Configure the required provider credentials and application settings according to the project's configuration.

---

# 🧪 Example

Once running, you can ask Vohu questions that require actual system interaction.

```text
> Inspect this project and tell me why the tests are failing.
```

Instead of simply generating an answer, the agent can:

```text
list_directory
       ↓
find_files
       ↓
read_file
       ↓
execute_command
       ↓
read_file
       ↓
final response
```

Another example:

```text
> Check the disk usage on my server.
```

The agent can reason through:

```text
ssh_execute
    ↓
df
    ↓
ssh_execute
    ↓
du
    ↓
analysis
```

The model handles the reasoning.

Vohu handles the capabilities and boundaries.

---

# 🧠 Why Build Another Agent?

There are already many AI assistants.

Vohu explores a slightly different problem:

### How do we build the runtime that sits between an LLM and a real system?

That introduces questions beyond ordinary chat:

* What is the model allowed to execute?
* How are tools represented across different providers?
* How do we prevent filesystem escapes?
* How do we constrain shell access?
* How should remote execution be controlled?
* What happens when a tool fails?
* How do we keep tool results from consuming the entire context?
* How should different LLM APIs expose the same capability?
* How can the same agent core power both CLI and web clients?

Vohu is an attempt to solve these problems in a small, understandable Go codebase.

---

# 🎯 Design Principles

### Provider-agnostic

The agent should not be tied to one LLM vendor.

### Tool-first

Capabilities should be explicit, structured, and independently executable.

### Controlled execution

LLMs should not receive unrestricted access to the host.

### Small core

The agent loop should remain understandable even as capabilities grow.

### Self-hosted

The runtime should be deployable under the user's own infrastructure.

### Real-world interaction

The purpose of tools is to interact with real systems, not only generate text.

---

# 🗺️ Roadmap

Vohu is actively evolving.

Planned or potential improvements include:

* [ ] Parallel tool execution
* [ ] Context management and compaction
* [ ] Dynamic tool discovery
* [ ] Better tool error semantics
* [ ] Human approval workflows
* [ ] Git integration
* [ ] Docker integration
* [ ] Kubernetes integration
* [ ] Background tasks
* [ ] Scheduled agents
* [ ] Agent observability and tracing
* [ ] Additional infrastructure integrations
* [ ] MCP integration

The focus is on improving the **agent runtime** rather than simply accumulating features.

---

# 🤝 Contributing

Contributions are welcome.

When adding a new capability, prefer creating an independent tool or provider adapter rather than coupling it directly to the agent loop.

For larger architectural changes, opening an issue or discussion first is recommended.

---

# 📜 License

See the repository license for details.

---

<p align="center">
  <sub>Built with Go · Designed for real systems · Powered by AI</sub>
</p>