import { createContext, useContext, useEffect, useState, type ReactNode } from "react"
import { api, levelSatisfies, type AccessLevel, type MyAccessDto } from "@/lib/api"
import { useAuth } from "@/lib/auth"

interface AccessContextValue {
  loading: boolean
  resourceAccess: MyAccessDto["resourceAccess"]
  hasLevel: (resourceType: string, level: AccessLevel) => boolean
  hasLevelOnResource: (resourceType: string, resourceId: number, level: AccessLevel) => boolean
}

const AccessContext = createContext<AccessContextValue | null>(null)

// AccessProvider fetches the caller's own resource-access profile once
// right after login (GET /me/access) and again on every login/logout
// transition — this is a UX layer for deciding what to show (nav items,
// buttons, route guards), not the actual security boundary; every endpoint
// still enforces its own access independently on the backend regardless of
// what this reports.
export function AccessProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  const [access, setAccess] = useState<MyAccessDto | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!isAuthenticated) {
      setAccess(null)
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)
    api.me
      .access()
      .then((result) => {
        if (!cancelled) setAccess(result)
      })
      .catch(() => {
        if (!cancelled) setAccess(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [isAuthenticated])

  const resourceAccess = access?.resourceAccess ?? []

  function hasLevel(resourceType: string, level: AccessLevel) {
    return levelSatisfies(access?.levels[resourceType], level)
  }

  // hasLevel alone only sees a *wildcard* grant (one that applies to
  // every resource of a type — see MyAccessDto's doc comment). A grant
  // scoped to one specific row (e.g. "manage" on ssh_connection #5, not
  // every connection) never shows up there, so a per-row action (open
  // terminal, edit, delete on *that* row) needs to also check
  // resourceAccess itself. A "prohibited" row is an explicit carve-out
  // and wins over any "accepted" one for the same exact resource — same
  // precedence as rbac.Service.findGrant on the backend, simplified
  // since this is only a display decision (the backend enforces the
  // real boundary independently regardless of what this reports).
  function hasLevelOnResource(resourceType: string, resourceId: number, level: AccessLevel) {
    if (hasLevel(resourceType, level)) return true

    const rows = resourceAccess.filter((r) => r.resourceType === resourceType && r.resourceId === resourceId)
    if (rows.some((r) => r.effect === "prohibited")) return false
    return rows.some((r) => r.effect === "accepted" && levelSatisfies(r.level, level))
  }

  return (
    <AccessContext.Provider value={{ loading, resourceAccess, hasLevel, hasLevelOnResource }}>
      {children}
    </AccessContext.Provider>
  )
}

export function useAccess() {
  const ctx = useContext(AccessContext)
  if (!ctx) throw new Error("useAccess must be used within an AccessProvider")
  return ctx
}
