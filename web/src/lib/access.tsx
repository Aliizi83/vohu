import { createContext, useContext, useEffect, useState, type ReactNode } from "react"
import { api, levelSatisfies, type AccessLevel, type MyAccessDto } from "@/lib/api"
import { useAuth } from "@/lib/auth"

interface AccessContextValue {
  loading: boolean
  resourceAccess: MyAccessDto["resourceAccess"]
  hasLevel: (resourceType: string, level: AccessLevel) => boolean
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

  return (
    <AccessContext.Provider value={{ loading, resourceAccess, hasLevel }}>
      {children}
    </AccessContext.Provider>
  )
}

export function useAccess() {
  const ctx = useContext(AccessContext)
  if (!ctx) throw new Error("useAccess must be used within an AccessProvider")
  return ctx
}
