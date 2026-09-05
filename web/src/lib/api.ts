// Typed client for Vohu's platform API (internal/platform/*, see
// cmd/server/main.go for the route list). Every response is wrapped in
// shared.BaseResponse{result, success, resultCode, validationErrors, error}
// — see internal/platform/shared/response.go.

const API_BASE = "/api/v1"

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

export interface SSHConnectionDto {
  id: number
  name: string
  host: string
  port: number
  username: string
  authMethod: "password" | "private_key"
  createdByUserId: number
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

export interface PermissionDto {
  id: number
  key: string
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
    grantPermission: (roleId: number, permissionId: number) =>
      request<null>("POST", `/roles/${roleId}/permissions`, { body: { permissionId } }),
  },

  permissions: {
    list: (page?: number, pageSize?: number, filter?: DynamicFilter) =>
      request<PagedList<PermissionDto>>("GET", "/permissions", { query: listQuery(page, pageSize, filter) }),
    create: (key: string) => request<PermissionDto>("POST", "/permissions", { body: { key } }),
    update: (id: number, key: string) => request<PermissionDto>("PUT", `/permissions/${id}`, { body: { key } }),
    remove: (id: number) => request<null>("DELETE", `/permissions/${id}`),
  },

  conversations: {
    list: (page?: number, pageSize?: number) =>
      request<PagedList<ConversationDto>>("GET", "/conversations", { query: listQuery(page, pageSize) }),
    create: (data: { title: string; provider: string; model: string }) =>
      request<ConversationDto>("POST", "/conversations", { body: data }),
    messages: (id: number) => request<MessageDto[]>("GET", `/conversations/${id}/messages`),
  },

  sshConnections: {
    list: (page?: number, pageSize?: number) =>
      request<PagedList<SSHConnectionDto>>("GET", "/ssh-connections", { query: listQuery(page, pageSize) }),
    create: (data: {
      name: string
      host: string
      port?: number
      username: string
      authMethod: "password" | "private_key"
      secret: string
    }) => request<SSHConnectionDto>("POST", "/ssh-connections", { body: data }),
    remove: (id: number) => request<null>("DELETE", `/ssh-connections/${id}`),
  },
}

export interface StreamHandlers {
  onChunk?: (text: string) => void
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
      else if (event === "done") handlers.onDone?.(payload as MessageDto[])
      else if (event === "error") handlers.onError?.(payload as string)
    }
  }
}
