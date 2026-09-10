import type { ReactNode } from "react"
import { ThemeProvider } from "next-themes"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import type { AccessLevel } from "@/lib/api"
import { AccessProvider, useAccess } from "@/lib/access"
import { AuthProvider, useAuth } from "@/lib/auth"
import { LanguageProvider } from "@/lib/i18n"
import AgentToolsPage from "@/pages/AgentToolsPage"
import ApiKeysPage from "@/pages/ApiKeysPage"
import ChatPage from "@/pages/ChatPage"
import Layout from "@/pages/Layout"
import LoginPage from "@/pages/LoginPage"
import ResourceAccessPage from "@/pages/ResourceAccessPage"
import RolesPage from "@/pages/RolesPage"
import SSHConnectionsPage from "@/pages/SSHConnectionsPage"
import UsersPage from "@/pages/UsersPage"

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

// PermissionRoute is a frontend-only convenience — it redirects away from
// a page the user has no use for (matching what the nav already hides)
// rather than rendering a page whose every API call would just 403. The
// real boundary is still enforced by each backend endpoint independently;
// this never substitutes for it. Renders nothing while access is still
// loading rather than flashing the redirect.
function PermissionRoute({
  resourceType,
  level,
  children,
}: {
  resourceType: string
  level: AccessLevel
  children: ReactNode
}) {
  const { hasLevel, loading } = useAccess()
  if (loading) return null
  if (!hasLevel(resourceType, level)) return <Navigate to="/chat" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem storageKey="vohu.theme">
      <LanguageProvider>
        <AuthProvider>
          <AccessProvider>
            <BrowserRouter>
              <Routes>
                <Route path="/login" element={<LoginPage />} />
                <Route
                  path="/"
                  element={
                    <ProtectedRoute>
                      <Layout />
                    </ProtectedRoute>
                  }
                >
                  <Route index element={<Navigate to="/chat" replace />} />
                  <Route path="chat" element={<ChatPage />} />
                  <Route path="ssh-connections" element={<SSHConnectionsPage />} />
                  <Route path="agent-tools" element={<AgentToolsPage />} />
                  <Route path="provider-keys" element={<ApiKeysPage />} />
                  <Route
                    path="users"
                    element={
                      <PermissionRoute resourceType="user" level="read">
                        <UsersPage />
                      </PermissionRoute>
                    }
                  />
                  <Route
                    path="roles"
                    element={
                      <PermissionRoute resourceType="role" level="manage">
                        <RolesPage />
                      </PermissionRoute>
                    }
                  />
                  <Route
                    path="resource-access"
                    element={
                      <PermissionRoute resourceType="resource_access" level="manage">
                        <ResourceAccessPage />
                      </PermissionRoute>
                    }
                  />
                </Route>
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </BrowserRouter>
            <Toaster />
          </AccessProvider>
        </AuthProvider>
      </LanguageProvider>
    </ThemeProvider>
  )
}
