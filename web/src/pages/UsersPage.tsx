import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useConfirm } from "@/components/ConfirmDialog"
import { SearchInput } from "@/components/SearchInput"
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
import { useLanguage } from "@/lib/i18n"
import { useDebouncedValue } from "@/lib/useDebouncedValue"
import { api, ApiError, type RoleDto, type UserDto } from "@/lib/api"

export default function UsersPage() {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()
  const [users, setUsers] = useState<UserDto[] | null>(null)
  const [roles, setRoles] = useState<RoleDto[]>([])
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const load = useCallback(async () => {
    try {
      const [userPage, rolePage] = await Promise.all([
        api.users.list(
          1,
          50,
          debouncedSearch ? { filters: { Username: { type: "contains", from: debouncedSearch } } } : undefined,
        ),
        api.roles.list(1, 50),
      ])
      setUsers(userPage.items)
      setRoles(rolePage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("users.loadFailed"))
    }
    // t intentionally excluded — its identity changing on language switch
    // shouldn't re-trigger a network call.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleToggleEnabled(user: UserDto) {
    try {
      await api.users.update(user.id, { enabled: !user.enabled })
      toast.success(t(user.enabled ? "users.disabledToast" : "users.enabledToast", { username: user.username }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("users.updateFailed"))
    }
  }

  async function handleDelete(user: UserDto) {
    const ok = await confirm({ description: t("users.confirmDelete", { username: user.username }) })
    if (!ok) return
    try {
      await api.users.remove(user.id)
      toast.success(t("users.deleted", { username: user.username }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("users.deleteFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("users.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("users.subtitle")}</p>
        </div>
        <CreateUserDialog onCreated={load} />
      </div>

      <SearchInput value={search} onChange={setSearch} placeholder={t("users.searchPlaceholder")} className="max-w-sm" />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("users.columnUsername")}</TableHead>
              <TableHead>{t("users.columnEmail")}</TableHead>
              <TableHead>{t("users.columnStatus")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
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
                  {debouncedSearch ? t("common.noSearchResults") : t("users.empty")}
                </TableCell>
              </TableRow>
            )}

            {users?.map((user) => (
              <TableRow key={user.id}>
                <TableCell className="font-medium">{user.username}</TableCell>
                <TableCell className="text-muted-foreground">{user.email || "—"}</TableCell>
                <TableCell>
                  <Badge variant={user.enabled ? "default" : "secondary"}>
                    {user.enabled ? t("users.enabled") : t("users.disabled")}
                  </Badge>
                </TableCell>
                <TableCell className="flex justify-end gap-2">
                  <AssignRoleDialog user={user} roles={roles} onAssigned={load} />
                  <Button variant="outline" size="sm" onClick={() => handleToggleEnabled(user)}>
                    {user.enabled ? t("users.disable") : t("users.enable")}
                  </Button>
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(user)}>
                    {t("common.delete")}
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
  const { t } = useLanguage()
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
      toast.success(t("users.created", { username }))
      setOpen(false)
      setUsername("")
      setEmail("")
      setPassword("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("users.createFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("users.newUser")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("users.createDialogTitle")}</DialogTitle>
            <DialogDescription>{t("users.createDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="new-username">{t("users.columnUsername")}</Label>
              <Input
                id="new-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                minLength={3}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-email">{t("users.emailOptional")}</Label>
              <Input
                id="new-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-password">{t("login.password")}</Label>
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
              {loading ? t("common.creating") : t("common.create")}
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
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [roleId, setRoleId] = useState<string>("")
  const [loading, setLoading] = useState(false)

  async function handleAssign() {
    if (!roleId) return
    setLoading(true)
    try {
      await api.users.assignRole(user.id, Number(roleId))
      toast.success(t("users.assigned", { username: user.username }))
      setOpen(false)
      setRoleId("")
      onAssigned()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("users.assignFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            {t("users.assignRole")}
          </Button>
        }
      />
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("users.assignDialogTitle", { username: user.username })}</DialogTitle>
          <DialogDescription>{t("users.assignDialogDescription")}</DialogDescription>
        </DialogHeader>
        <div className="py-4">
          <Select value={roleId} onValueChange={(value) => setRoleId(value ?? "")}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("users.choosePermissionRole")}>
                {(value: string) => roles.find((r) => String(r.id) === value)?.name ?? t("users.choosePermissionRole")}
              </SelectValue>
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
            {loading ? t("users.assigning") : t("users.assign")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
