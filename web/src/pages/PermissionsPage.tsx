import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { api, ApiError, type PermissionDto } from "@/lib/api"

export default function PermissionsPage() {
  const [permissions, setPermissions] = useState<PermissionDto[] | null>(null)

  const load = useCallback(async () => {
    try {
      const page = await api.permissions.list(1, 200)
      setPermissions(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load permissions")
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(permission: PermissionDto) {
    if (!confirm(`Delete permission "${permission.key}"? This can't be undone.`)) return
    try {
      await api.permissions.remove(permission.id)
      toast.success(`Permission "${permission.key}" deleted`)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Delete failed")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Permissions</h2>
          <p className="text-sm text-muted-foreground">
            Flat keys like <code className="rounded bg-muted px-1 py-0.5 text-xs">user:create</code> —
            granted to roles on the Roles page.
          </p>
        </div>
        <CreatePermissionDialog onCreated={load} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Key</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {permissions === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={2}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {permissions?.length === 0 && (
              <TableRow>
                <TableCell colSpan={2} className="text-center text-muted-foreground">
                  No permissions yet.
                </TableCell>
              </TableRow>
            )}

            {permissions?.map((permission) => (
              <TableRow key={permission.id}>
                <TableCell className="font-mono text-sm">{permission.key}</TableCell>
                <TableCell className="text-right">
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(permission)}>
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

function CreatePermissionDialog({ onCreated }: { onCreated: () => void }) {
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.permissions.create(key)
      toast.success(`Permission "${key}" created`)
      setOpen(false)
      setKey("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create permission")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>New permission</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create permission</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="permission-key">Key</Label>
            <Input
              id="permission-key"
              placeholder="resource:action"
              value={key}
              onChange={(e) => setKey(e.target.value)}
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
