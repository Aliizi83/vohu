import { KeyRound, KeySquare, LogOut, MessageSquare, Server, Shield, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth"
import { LANGUAGE_OPTIONS, useLanguage } from "@/lib/i18n"
import { cn } from "cn"

const navItems = [
  { to: "/chat", labelKey: "nav.chat", icon: MessageSquare },
  { to: "/ssh-connections", labelKey: "nav.sshConnections", icon: Server },
  { to: "/provider-keys", labelKey: "nav.apiKeys", icon: KeySquare },
  { to: "/users", labelKey: "nav.users", icon: Users },
  { to: "/roles", labelKey: "nav.roles", icon: Shield },
  { to: "/permissions", labelKey: "nav.permissions", icon: KeyRound },
] as const

export default function Layout() {
  const { logout } = useAuth()
  const { t, language, setLanguage } = useLanguage()

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col gap-1 border-e bg-card p-4">
        <div className="mb-4 px-2">
          <h1 className="text-lg font-semibold">{t("nav.appName")}</h1>
          <p className="text-xs text-muted-foreground">{t("nav.appTagline")}</p>
        </div>

        {navItems.map(({ to, labelKey, icon: Icon }) => (
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
