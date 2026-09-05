import { createContext, useContext, useEffect, useState, type ReactNode } from "react"
import { api, getAccessToken, setAccessToken } from "@/lib/api"

interface AuthContextValue {
  isAuthenticated: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isAuthenticated, setIsAuthenticated] = useState(() => Boolean(getAccessToken()))

  useEffect(() => {
    // api.ts fires this when any request comes back 401 (token missing,
    // expired, or invalid) — drop back to the login screen from wherever
    // the user happened to be.
    function handleUnauthorized() {
      setIsAuthenticated(false)
    }
    window.addEventListener("vohu:unauthorized", handleUnauthorized)
    return () => window.removeEventListener("vohu:unauthorized", handleUnauthorized)
  }, [])

  async function login(username: string, password: string) {
    const tokens = await api.login(username, password)
    setAccessToken(tokens.accessToken)
    setIsAuthenticated(true)
  }

  function logout() {
    setAccessToken(null)
    setIsAuthenticated(false)
  }

  return (
    <AuthContext.Provider value={{ isAuthenticated, login, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider")
  return ctx
}
