// Typed client for Vohu's platform API (internal/platform/*, see
// cmd/server/main.go for the route list). Every response is wrapped in
// shared.BaseResponse{result, success, resultCode, validationErrors, error}
// — see internal/platform/shared/response.go.

export const API_BASE = "/api/v1"

export class ApiError extends Error {
  status: number
  resultCode?: number
  validationErrors?: { property: string; tag: string; message: string }[]

  constructor(
    status: number,
    message: string,
    resultCode?: number,
    validationErrors?: { property: string; tag: string; message: string }[],
  ) {
    super(message)
    this.status = status
    this.resultCode = resultCode
    this.validationErrors = validationErrors
  }
}

let accessToken: string | null = localStorage.getItem("vohu.accessToken")

export function setAccessToken(token: string | null) {
  accessToken = token
  if (token) localStorage.setItem("vohu.accessToken", token)
  else localStorage.removeItem("vohu.accessToken")
}

export function getAccessToken() {
  return accessToken
}

interface BaseResponse<T> {
  result: T
  success: boolean
  resultCode: number
  validationErrors?: { property: string; tag: string; message: string }[]
  error?: string
}

async function request<T>(
  method: string,
  path: string,
  opts?: { body?: unknown; query?: Record<string, string | undefined> },
): Promise<T> {
  const url = new URL(API_BASE + path, window.location.origin)
  if (opts?.query) {
    for (const [key, value] of Object.entries(opts.query)) {
      if (value !== undefined && value !== "") url.searchParams.set(key, value)
    }
  }

  const headers: Record<string, string> = {}
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`

  let body: string | undefined
  if (opts?.body !== undefined) {
    headers["Content-Type"] = "application/json"
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(url.toString(), { method, headers, body })

  // A 204/empty body (shouldn't happen — every handler responds through
  // BaseResponse — but guard anyway) still needs a safe fallback.
  const json = (await res.json().catch(() => null)) as BaseResponse<T> | null

  if (res.status === 401) {
    setAccessToken(null)
    window.dispatchEvent(new Event("vohu:unauthorized"))
  }

  if (!res.ok || !json?.success) {
    throw new ApiError(
      res.status,
      json?.error ?? res.statusText,
      json?.resultCode,
      json?.validationErrors,
    )
  }

  return json.result
}

export interface TokenPair {
  accessToken: string
  accessTokenExpiresAt: number
  refreshToken: string
  refreshTokenExpiresAt: number
}

export interface ConversationDto {
  id: number
  title: string
  provider: string
  model: string
  customModelId?: number
  archived: boolean
}

export interface ToolCallDto {
  id: string
  name: string
  arguments: Record<string, unknown>
}

export interface ToolResultDto {
  toolCallId: string
  name: string
  result: unknown
  error?: string
}

export interface MessageDto {
  role: "user" | "assistant" | "tool"
  content?: string
  toolCalls?: ToolCallDto[]
  toolResults?: ToolResultDto[]
}

export type CommandPolicyMode = "accept" | "prohibited"

export interface SSHConnectionDto {
  id: number
  name: string
  host: string
  port: number
  username: string
  // "accept" (default): a command not matched by any rule below is
  // denied — the connection's commandRules are an allow-list. "prohibited"
  // inverts that: every command is allowed except one matched by a rule
  // — the same rules become a deny-list instead.
  commandPolicyMode: CommandPolicyMode
  createdByUserId: number
}

export type LLMProvider = "gemini" | "anthropic" | "openai"

export interface ProviderKeyDto {
  provider: LLMProvider
  baseUrl?: string
  workspaceId?: string
}

// CustomModelDto is one named OpenAI-compatible preset (see
// custommodel.CustomModel on the backend) — unlike ProviderKeyDto there
// can be any number of these per user, which is the point: two different
// OpenAI-compatible endpoints (a local Ollama server and a DeepSeek
// account, say) need two different (baseUrl, apiKey) pairs, not one.
export interface CustomModelDto {
  id: number
  name: string
  baseUrl: string
  modelName: string
}

// CommandRuleDto is one entry of an SSH connection's own command rule
// list (see internal/platform/commandrule.Rule) — evaluated first-match-
// wins; whether an unmatched command is denied or allowed depends on the
// connection's own commandPolicyMode (SSHConnectionDto). argsPrefixes is
// an OR of prefixes: an empty array matches any args at all, a non-empty
// one requires the command's args to start with one of the listed word
// sequences.
export interface CommandRuleDto {
  id: number
  sshConnectionId: number
  program: string
  argsPrefixes: string[][]
  allowed: boolean
}

export type AgentToolVisibility = "public" | "private"

// AgentToolDto is one entry in the fixed, built-in tool catalog (see
// internal/platform/agenttool.Tool) — ssh_execute and
// list_ssh_connections, the only two tools that aren't user/agent-defined
// (see CustomToolDto for those). Every tool here is SSH-connection-bound:
// it takes a connectionId at execution time, no exceptions.
export interface AgentToolDto {
  id: number
  name: string
  description: string
  visibility: AgentToolVisibility
  implemented: boolean
}

export type CustomToolVisibility = "public" | "private"

// CustomToolDto is one user/agent-authored tool definition (see
// internal/platform/customtool.Tool) — its actual behavior lives on its
// versions (see CustomToolVersionDto), built and deployed on demand to
// whichever SSH connection a call names.
export interface CustomToolDto {
  id: number
  name: string
  description: string
  paramsSchema: string
  visibility: CustomToolVisibility
  createdByUserId: number
}

export interface CustomToolVersionDto {
  id: number
  toolId: number
  version: string
  sourceCode: string
  createdByUserId: number
}

export interface SourceDiagnosticDto {
  line: number
  column: number
  message: string
}

export interface CheckSourceResultDto {
  success: boolean
  diagnostics: SourceDiagnosticDto[]
}

export type AccessLevel = "read" | "write" | "manage"
export type GranteeType = "user" | "role"
export type ResourceEffect = "accepted" | "prohibited"

export interface ResourceAccessDto {
  id: number
  granteeType: GranteeType
  granteeId: number
  resourceType: string
  resourceId: number
  level: AccessLevel
  effect: ResourceEffect
}

export interface UserDto {
  id: number
  username: string
  email: string
  enabled: boolean
}

export interface RoleDto {
  id: number
  name: string
}

// MyAccessDto is the caller's own complete access profile, fetched once
// right after login. ResourceAccess is every grant that's personally
// theirs (direct or via a role they hold); Levels is the best level they
// hold on each known resource type at large (checked against the
// wildcard resource) — what drives nav-item and button visibility, since
// the frontend can't enumerate every resource ID up front. The backend
// still enforces every boundary independently regardless of what this
// reports — this is a UX layer, not the actual security boundary.
export interface MyAccessDto {
  resourceAccess: ResourceAccessDto[]
  levels: Partial<Record<string, AccessLevel>>
}

// LEVEL_RANK/hasLevel mirror rbac.AccessLevel.Satisfies on the Go side —
// "does the caller's best level on this resource type meet or exceed
// what's required" (e.g. a caller with "manage" satisfies a "read" check).
const LEVEL_RANK: Record<AccessLevel, number> = { read: 1, write: 2, manage: 3 }

export function levelSatisfies(level: AccessLevel | undefined, required: AccessLevel): boolean {
  if (!level) return false
  return LEVEL_RANK[level] >= LEVEL_RANK[required]
}

export interface PagedList<T> {
  pageNumber: number
  pageSize: number
  totalRows: number
  totalPages: number
  hasPreviousPage: boolean
  hasNextPage: boolean
  items: T[]
}

export interface DynamicFilter {
  filters?: Record<string, { type: string; from: string; to?: string }>
  sorts?: { columnId: string; sort: "asc" | "desc" }[]
}

function listQuery(page?: number, pageSize?: number, filter?: DynamicFilter) {
  const query: Record<string, string | undefined> = {
    pageNumber: page?.toString(),
    pageSize: pageSize?.toString(),
  }
  if (filter && (filter.filters || filter.sorts)) {
    query.filter = JSON.stringify(filter)
  }
  return query
}

export const api = {
  login: (username: string, password: string) =>
    request<TokenPair>("POST", "/auth/login", { body: { username, password } }),

  me: {
    access: () => request<MyAccessDto>("GET", "/me/access"),
  },

  users: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<UserDto>>("GET", "/users", { query: listQuery(page, pageSize, filter) }),
    get: (id: number) => request<UserDto>("GET", `/users/${id}`),
    create: (data: { username: string; email?: string; password: string }) =>
      request<UserDto>("POST", "/users", { body: data }),
    update: (id: number, data: { email?: string; enabled?: boolean }) =>
      request<UserDto>("PUT", `/users/${id}`, { body: data }),
    remove: (id: number) => request<null>("DELETE", `/users/${id}`),
    assignRole: (userId: number, roleId: number) =>
      request<null>("POST", `/users/${userId}/roles`, { body: { roleId } }),
  },

  roles: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<RoleDto>>("GET", "/roles", { query: listQuery(page, pageSize, filter) }),
    get: (id: number) => request<RoleDto>("GET", `/roles/${id}`),
    create: (name: string) => request<RoleDto>("POST", "/roles", { body: { name } }),
    update: (id: number, name: string) => request<RoleDto>("PUT", `/roles/${id}`, { body: { name } }),
    remove: (id: number) => request<null>("DELETE", `/roles/${id}`),
  },

  conversations: {
    list: (page?: number, pageSize?: number, archived?: boolean) =>
      request<PagedList<ConversationDto>>("GET", "/conversations", {
        query: { ...listQuery(page, pageSize), archived: archived ? "true" : undefined },
      }),
    // No title: the server names a new conversation generically and
    // renames it from the first message once one's actually sent (see
    // chat.Handler.SendMessage) — the same "picks a name for you" feel
    // as ChatGPT and friends, without asking upfront for a name for a
    // conversation that doesn't have any content yet.
    create: (data: { provider: string; model: string; customModelId?: number }) =>
      request<ConversationDto>("POST", "/conversations", { body: data }),
    // provider/model/customModelId travel together — set provider+model to
    // switch which model this conversation talks to going forward (omit
    // customModelId to clear it, e.g. switching back to a built-in model).
    update: (
      id: number,
      data: { title?: string; archived?: boolean; provider?: string; model?: string; customModelId?: number },
    ) => request<ConversationDto>("PUT", `/conversations/${id}`, { body: data }),
    remove: (id: number) => request<null>("DELETE", `/conversations/${id}`),
    messages: (id: number, page?: number, pageSize?: number) =>
      request<PagedList<MessageDto>>("GET", `/conversations/${id}/messages`, { query: listQuery(page, pageSize) }),
  },

  sshConnections: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<SSHConnectionDto>>("GET", "/ssh-connections", {
        query: listQuery(page, pageSize, filter),
      }),
    create: (data: {
      name: string
      host: string
      port?: number
      username: string
      privateKey: string
    }) => request<SSHConnectionDto>("POST", "/ssh-connections", { body: data }),
    update: (
      id: number,
      data: {
        name?: string
        host?: string
        port?: number
        username?: string
        // Omitted (or empty) keeps the existing key — there is no way to
        // show it back from the server to prefill a form.
        privateKey?: string
        commandPolicyMode?: CommandPolicyMode
      },
    ) => request<SSHConnectionDto>("PUT", `/ssh-connections/${id}`, { body: data }),
    remove: (id: number) => request<null>("DELETE", `/ssh-connections/${id}`),
    // One-time, 30s-lived ticket for the web terminal's WebSocket — see
    // terminal.Handler.ServeWS's doc comment for why the socket itself
    // can't just carry the normal Bearer token.
    terminalTicket: (id: number) => request<{ ticket: string }>("POST", `/ssh-connections/${id}/terminal-ticket`),
  },

  providerKeys: {
    listMine: () => request<ProviderKeyDto[]>("GET", "/provider-keys/me"),
    setMine: (data: { provider: LLMProvider; apiKey: string; baseUrl?: string; workspaceId?: string }) =>
      request<null>("POST", "/provider-keys/me", { body: data }),
    removeMine: (provider: LLMProvider) => request<null>("DELETE", `/provider-keys/me/${provider}`),

    listGlobal: () => request<ProviderKeyDto[]>("GET", "/provider-keys"),
    setGlobal: (data: { provider: LLMProvider; apiKey: string; baseUrl?: string; workspaceId?: string }) =>
      request<null>("POST", "/provider-keys", { body: data }),
    removeGlobal: (provider: LLMProvider) => request<null>("DELETE", `/provider-keys/${provider}`),
  },

  customModels: {
    // The new-chat picker's read path — every authenticated user's own
    // presets plus every global one, regardless of whether they're
    // allowed to manage (create/delete) global presets.
    listAvailable: () => request<CustomModelDto[]>("GET", "/custom-models/available"),

    listMine: () => request<CustomModelDto[]>("GET", "/custom-models/me"),
    createMine: (data: { name: string; baseUrl: string; modelName: string; apiKey: string }) =>
      request<CustomModelDto>("POST", "/custom-models/me", { body: data }),
    removeMine: (id: number) => request<null>("DELETE", `/custom-models/me/${id}`),

    listGlobal: () => request<CustomModelDto[]>("GET", "/custom-models"),
    createGlobal: (data: { name: string; baseUrl: string; modelName: string; apiKey: string }) =>
      request<CustomModelDto>("POST", "/custom-models", { body: data }),
    removeGlobal: (id: number) => request<null>("DELETE", `/custom-models/${id}`),
  },

  commandRules: {
    // Rules are always fetched scoped to one connection — there's no
    // "list every rule" endpoint, matching commandrule.Handler.List on the
    // backend (it requires sshConnectionId as a query param).
    list: (sshConnectionId: number) =>
      request<PagedList<CommandRuleDto>>("GET", "/command-rules", {
        query: { sshConnectionId: String(sshConnectionId), pageSize: "100" },
      }),
    create: (data: {
      sshConnectionId: number
      program: string
      argsPrefixes?: string[][]
      allowed?: boolean
    }) => request<CommandRuleDto>("POST", "/command-rules", { body: data }),
    update: (id: number, data: { allowed?: boolean }) =>
      request<CommandRuleDto>("PUT", `/command-rules/${id}`, { body: data }),
    remove: (id: number) => request<null>("DELETE", `/command-rules/${id}`),
  },

  agentTools: {
    // Every public tool plus whatever private ones the caller holds at
    // least Read-level resource access to — see
    // agenttool.Service.ListForCaller.
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<AgentToolDto>>("GET", "/agent-tools", {
        query: listQuery(page, pageSize, filter),
      }),
    setVisibility: (id: number, visibility: AgentToolVisibility) =>
      request<AgentToolDto>("PUT", `/agent-tools/${id}`, { body: { visibility } }),
  },

  customTools: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<CustomToolDto>>("GET", "/custom-tools", {
        query: listQuery(page, pageSize, filter),
      }),
    create: (data: { name: string; description: string; paramsSchema: string; visibility?: CustomToolVisibility }) =>
      request<CustomToolDto>("POST", "/custom-tools", { body: data }),
    update: (id: number, data: { description?: string; paramsSchema?: string; visibility?: CustomToolVisibility }) =>
      request<CustomToolDto>("PUT", `/custom-tools/${id}`, { body: data }),
    remove: (id: number) => request<null>("DELETE", `/custom-tools/${id}`),
    listVersions: (toolId: number, page?: number, pageSize?: number) =>
      request<PagedList<CustomToolVersionDto>>("GET", `/custom-tools/${toolId}/versions`, {
        query: listQuery(page, pageSize),
      }),
    createVersion: (toolId: number, data: { version: string; sourceCode: string }) =>
      request<CustomToolVersionDto>("POST", `/custom-tools/${toolId}/versions`, { body: data }),
    checkSource: (sourceCode: string) =>
      request<CheckSourceResultDto>("POST", "/custom-tools/check-source", { body: { sourceCode } }),
  },

  resourceAccess: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<ResourceAccessDto>>("GET", "/resource-access", {
        query: listQuery(page, pageSize, filter),
      }),
    grant: (data: {
      granteeType: GranteeType
      granteeId: number
      resourceType: string
      resourceId: number
      level: AccessLevel
      effect: ResourceEffect
    }) => request<null>("POST", "/resource-access", { body: data }),
    revoke: (id: number) => request<null>("DELETE", `/resource-access/${id}`),
  },
}

export interface StreamHandlers {
  onChunk?: (text: string) => void
  onToolCall?: (call: ToolCallDto) => void
  onToolResult?: (result: ToolResultDto) => void
  // Fired only for a conversation's first message, once the server has
  // renamed it from its initial generic title to something derived from
  // that message — see chat.Handler.SendMessage.
  onTitleChanged?: (title: string) => void
  onDone?: (messages: MessageDto[]) => void
  onError?: (message: string) => void
}

// streamMessage hand-rolls SSE parsing over a plain fetch (rather than
// EventSource) because the endpoint is a POST that needs an Authorization
// header — EventSource only ever issues unauthenticated GETs, and putting
// the access token in the URL as a query param just to use it would leak
// the token into logs/history for no real benefit here.
export async function streamMessage(
  conversationId: number,
  content: string,
  handlers: StreamHandlers,
): Promise<void> {
  const headers: Record<string, string> = { "Content-Type": "application/json" }
  if (accessToken) headers.Authorization = `Bearer ${accessToken}`

  const res = await fetch(`${API_BASE}/conversations/${conversationId}/messages`, {
    method: "POST",
    headers,
    body: JSON.stringify({ content }),
  })

  if (res.status === 401) {
    setAccessToken(null)
    window.dispatchEvent(new Event("vohu:unauthorized"))
  }

  if (!res.ok || !res.body) {
    // The server rejected the request before ever switching into SSE mode
    // (validation, ownership, a missing LLM API key, ...) — a normal
    // BaseResponse JSON error, not an SSE stream.
    const json = (await res.json().catch(() => null)) as { error?: string } | null
    handlers.onError?.(json?.error ?? res.statusText)
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    let separatorIndex: number
    while ((separatorIndex = buffer.indexOf("\n\n")) !== -1) {
      const frame = buffer.slice(0, separatorIndex)
      buffer = buffer.slice(separatorIndex + 2)

      let event = "message"
      let data = ""
      for (const line of frame.split("\n")) {
        if (line.startsWith("event: ")) event = line.slice("event: ".length)
        else if (line.startsWith("data: ")) data = line.slice("data: ".length)
      }
      if (!data) continue

      const payload = JSON.parse(data) as unknown

      if (event === "chunk") handlers.onChunk?.(payload as string)
      else if (event === "tool_call") handlers.onToolCall?.(payload as ToolCallDto)
      else if (event === "tool_result") handlers.onToolResult?.(payload as ToolResultDto)
      else if (event === "title") handlers.onTitleChanged?.(payload as string)
      else if (event === "done") handlers.onDone?.(payload as MessageDto[])
      else if (event === "error") handlers.onError?.(payload as string)
    }
  }
}
