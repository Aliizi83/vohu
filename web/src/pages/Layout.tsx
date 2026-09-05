import { KeyRound, KeySquare, LogOut, MessageSquare, Server, Shield, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth"

const navItems = [
  { to: "/chat", label: "Chat", icon: MessageSquare },
  { to: "/ssh-connections", label: "SSH Connections", icon: Server },
  { to: "/provider-keys", label: "API Keys", icon: KeySquare },
  { to: "/users", label: "Users", icon: Users },
  { to: "/roles", label: "Roles", icon: Shield },
  { to: "/permissions", label: "Permissions", icon: KeyRound },
]

export default function Layout() {
  const { logout } = useAuth()

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col gap-1 border-r bg-card p-4">
        <div className="mb-4 px-2">
          <h1 className="text-lg font-semibold">Vohu</h1>
          <p className="text-xs text-muted-foreground">Platform admin</p>
        </div>

        {navItems.map(({ to, label, icon: Icon }) => (
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
            {label}
          </NavLink>
        ))}

        <div className="mt-auto pt-4">
          <Button variant="ghost" className="w-full justify-start gap-2" onClick={logout}>
            <LogOut className="size-4" />
            Log out
          </Button>
        </div>
      </aside>

      <main className="flex-1 overflow-x-auto p-6">
        <Outlet />
      </main>
    </div>
  )
}
