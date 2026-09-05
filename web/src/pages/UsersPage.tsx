import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { api, ApiError, type RoleDto, type UserDto } from "@/lib/api"

export default function UsersPage() {
  const [users, setUsers] = useState<UserDto[] | null>(null)
  const [roles, setRoles] = useState<RoleDto[]>([])

  const load = useCallback(async () => {
    try {
      const [userPage, rolePage] = await Promise.all([
        api.users.list(1, 50),
        api.roles.list(1, 50),
      ])
      setUsers(userPage.items)
      setRoles(rolePage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load users")
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function handleToggleEnabled(user: UserDto) {
    try {
      await api.users.update(user.id, { enabled: !user.enabled })
      toast.success(`${user.username} ${user.enabled ? "disabled" : "enabled"}`)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Update failed")
    }
  }

  async function handleDelete(user: UserDto) {
    if (!confirm(`Delete user "${user.username}"? This can't be undone.`)) return
    try {
      await api.users.remove(user.id)
      toast.success(`${user.username} deleted`)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Delete failed")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Users</h2>
          <p className="text-sm text-muted-foreground">Create accounts and manage their roles.</p>
        </div>
        <CreateUserDialog onCreated={load} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {users === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={4}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {users?.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  No users yet.
                </TableCell>
              </TableRow>
            )}

            {users?.map((user) => (
              <TableRow key={user.id}>
                <TableCell className="font-medium">{user.username}</TableCell>
                <TableCell className="text-muted-foreground">{user.email || "—"}</TableCell>
                <TableCell>
                  <Badge variant={user.enabled ? "default" : "secondary"}>
                    {user.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                </TableCell>
                <TableCell className="flex justify-end gap-2">
                  <AssignRoleDialog user={user} roles={roles} onAssigned={load} />
                  <Button variant="outline" size="sm" onClick={() => handleToggleEnabled(user)}>
                    {user.enabled ? "Disable" : "Enable"}
                  </Button>
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(user)}>
                    Delete
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function CreateUserDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.users.create({ username, email: email || undefined, password })
      toast.success(`User "${username}" created`)
      setOpen(false)
      setUsername("")
      setEmail("")
      setPassword("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create user")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>New user</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create user</DialogTitle>
            <DialogDescription>Password must be at least 8 characters.</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="new-username">Username</Label>
              <Input
                id="new-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                minLength={3}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-email">Email (optional)</Label>
              <Input
                id="new-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">Password</Label>
              <Input
                id="new-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={8}
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading}>
              {loading ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function AssignRoleDialog({
  user,
  roles,
  onAssigned,
}: {
  user: UserDto
  roles: RoleDto[]
  onAssigned: () => void
}) {
  const [open, setOpen] = useState(false)
  const [roleId, setRoleId] = useState<string>("")
  const [loading, setLoading] = useState(false)

  async function handleAssign() {
    if (!roleId) return
    setLoading(true)
    try {
      await api.users.assignRole(user.id, Number(roleId))
      toast.success(`Role assigned to ${user.username}`)
      setOpen(false)
      setRoleId("")
      onAssigned()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to assign role")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            Assign role
          </Button>
        }
      />
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Assign role to {user.username}</DialogTitle>
          <DialogDescription>Grants every permission attached to the chosen role.</DialogDescription>
        </DialogHeader>
        <div className="py-4">
          <Select value={roleId} onValueChange={(value) => setRoleId(value ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Choose a role" />
            </SelectTrigger>
            <SelectContent>
              {roles.map((role) => (
                <SelectItem key={role.id} value={String(role.id)}>
                  {role.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button onClick={handleAssign} disabled={!roleId || loading}>
            {loading ? "Assigning..." : "Assign"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
