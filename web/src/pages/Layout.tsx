import { KeyRound, KeySquare, LogOut, MessageSquare, Server, Shield, ShieldCheck, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAccess } from "@/lib/access"
import { useAuth } from "@/lib/auth"
import { LANGUAGE_OPTIONS, useLanguage } from "@/lib/i18n"
import { cn } from "cn"

// requiredPermission is undefined for items open to every authenticated
// user (Chat, SSH Connections — visibility of *records* there is filtered
// server-side per user rather than gated by a flat permission at all,
// API Keys — self-service). Items with one are hidden unless the user's
// /me/access profile includes that key, matching what would otherwise
// just 403.
const navItems = [
  { to: "/chat", labelKey: "nav.chat", icon: MessageSquare, requiredPermission: undefined },
  { to: "/ssh-connections", labelKey: "nav.sshConnections", icon: Server, requiredPermission: undefined },
  { to: "/provider-keys", labelKey: "nav.apiKeys", icon: KeySquare, requiredPermission: undefined },
  { to: "/users", labelKey: "nav.users", icon: Users, requiredPermission: "user:read" },
  { to: "/roles", labelKey: "nav.roles", icon: Shield, requiredPermission: "rbac:manage" },
  { to: "/permissions", labelKey: "nav.permissions", icon: KeyRound, requiredPermission: "rbac:manage" },
  { to: "/resource-access", labelKey: "nav.resourceAccess", icon: ShieldCheck, requiredPermission: "rbac:manage" },
] as const

export default function Layout() {
  const { logout } = useAuth()
  const { hasPermission } = useAccess()
  const { t, language, setLanguage } = useLanguage()

  const visibleNavItems = navItems.filter(
    (item) => item.requiredPermission === undefined || hasPermission(item.requiredPermission),
  )

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col gap-1 border-e bg-card p-4">
        <div className="mb-4 px-2">
          <h1 className="text-lg font-semibold">{t("nav.appName")}</h1>
          <p className="text-xs text-muted-foreground">{t("nav.appTagline")}</p>
        </div>

        {visibleNavItems.map(({ to, labelKey, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors ${
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
              }`
            }
          >
            <Icon className="size-4" />
            {t(labelKey)}
          </NavLink>
        ))}

        <div className="mt-auto space-y-3 pt-4">
          <div className="flex items-center gap-1 px-2">
            <span className="text-xs text-muted-foreground">{t("nav.language")}:</span>
            <div className="flex gap-1">
              {LANGUAGE_OPTIONS.map((option) => (
                <button
                  key={option.value}
                  onClick={() => setLanguage(option.value)}
                  className={cn(
                    "rounded-md px-2 py-0.5 text-xs transition-colors",
                    language === option.value
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <Button variant="ghost" className="w-full justify-start gap-2" onClick={logout}>
            <LogOut className="size-4" />
            {t("nav.logOut")}
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-x-auto p-6">
        <Outlet />
      </main>
    </div>
  )
}
