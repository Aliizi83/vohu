import type { ReactNode } from "react"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { AuthProvider, useAuth } from "@/lib/auth"
import { LanguageProvider } from "@/lib/i18n"
import ApiKeysPage from "@/pages/ApiKeysPage"
import ChatPage from "@/pages/ChatPage"
import Layout from "@/pages/Layout"
import LoginPage from "@/pages/LoginPage"
import PermissionsPage from "@/pages/PermissionsPage"
import RolesPage from "@/pages/RolesPage"
import SSHConnectionsPage from "@/pages/SSHConnectionsPage"
import UsersPage from "@/pages/UsersPage"

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth()
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <LanguageProvider>
      <AuthProvider>
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
              <Route path="provider-keys" element={<ApiKeysPage />} />
              <Route path="users" element={<UsersPage />} />
              <Route path="roles" element={<RolesPage />} />
              <Route path="permissions" element={<PermissionsPage />} />
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
        <Toaster />
      </AuthProvider>
    </LanguageProvider>
  )
}
