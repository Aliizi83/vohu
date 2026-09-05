import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
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
import { api, ApiError, type PermissionDto, type RoleDto } from "@/lib/api"

export default function RolesPage() {
  const [roles, setRoles] = useState<RoleDto[] | null>(null)
  const [permissions, setPermissions] = useState<PermissionDto[]>([])

  const load = useCallback(async () => {
    try {
      const [rolePage, permissionPage] = await Promise.all([
        api.roles.list(1, 50),
        api.permissions.list(1, 200),
      ])
      setRoles(rolePage.items)
      setPermissions(permissionPage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load roles")
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(role: RoleDto) {
    if (!confirm(`Delete role "${role.name}"? This can't be undone.`)) return
    try {
      await api.roles.remove(role.id)
      toast.success(`Role "${role.name}" deleted`)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Delete failed")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Roles</h2>
          <p className="text-sm text-muted-foreground">
            Roles bundle permissions; assign a role to a user on the Users page.
          </p>
        </div>
        <CreateRoleDialog onCreated={load} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {roles === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={2}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {roles?.length === 0 && (
              <TableRow>
                <TableCell colSpan={2} className="text-center text-muted-foreground">
                  No roles yet.
                </TableCell>
              </TableRow>
            )}

            {roles?.map((role) => (
              <TableRow key={role.id}>
                <TableCell className="font-medium">{role.name}</TableCell>
                <TableCell className="flex justify-end gap-2">
                  <GrantPermissionDialog role={role} permissions={permissions} onGranted={load} />
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(role)}>
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

function CreateRoleDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.roles.create(name)
      toast.success(`Role "${name}" created`)
      setOpen(false)
      setName("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create role")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>New role</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create role</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="role-name">Name</Label>
            <Input
              id="role-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              minLength={2}
            />
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

function GrantPermissionDialog({
  role,
  permissions,
  onGranted,
}: {
  role: RoleDto
  permissions: PermissionDto[]
  onGranted: () => void
}) {
  const [open, setOpen] = useState(false)
  const [permissionId, setPermissionId] = useState<string>("")
  const [loading, setLoading] = useState(false)

  async function handleGrant() {
    if (!permissionId) return
    setLoading(true)
    try {
      await api.roles.grantPermission(role.id, Number(permissionId))
      toast.success(`Permission granted to ${role.name}`)
      setOpen(false)
      setPermissionId("")
      onGranted()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to grant permission")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            Grant permission
          </Button>
        }
      />
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Grant permission to {role.name}</DialogTitle>
          <DialogDescription>
            Every user with this role gains this permission immediately.
          </DialogDescription>
        </DialogHeader>
        <div className="py-4">
          <Select value={permissionId} onValueChange={(value) => setPermissionId(value ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder="Choose a permission" />
            </SelectTrigger>
            <SelectContent>
              {permissions.map((permission) => (
                <SelectItem key={permission.id} value={String(permission.id)}>
                  {permission.key}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <DialogFooter>
          <Button onClick={handleGrant} disabled={!permissionId || loading}>
            {loading ? "Granting..." : "Grant"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
