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
}
