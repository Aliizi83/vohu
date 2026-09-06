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
import {
  api,
  ApiError,
  type AccessLevel,
  type ResourcePermissionDto,
  type SSHConnectionDto,
  type UserDto,
} from "@/lib/api"

// The only resource type the platform actually has today — a free string
// on the backend (sshconn is never imported by rbac), but there's nothing
// else to grant access to yet, so the picker doesn't need to be more
// flexible than this.
const RESOURCE_TYPE_SSH_CONNECTION = "ssh_connection"

const LEVEL_KEYS: Record<AccessLevel, string> = {
  forbidden: "resourceAccess.levelForbidden",
  read: "resourceAccess.levelRead",
  write: "resourceAccess.levelWrite",
  manage: "resourceAccess.levelManage",
}

export default function ResourceAccessPage() {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()
  const [grants, setGrants] = useState<ResourcePermissionDto[] | null>(null)
  const [users, setUsers] = useState<UserDto[]>([])
  const [connections, setConnections] = useState<SSHConnectionDto[]>([])
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const usersByID = new Map(users.map((u) => [u.id, u]))
  const connectionsByID = new Map(connections.map((c) => [c.id, c]))

  const load = useCallback(async () => {
    try {
      const [grantPage, userPage, connectionPage] = await Promise.all([
        api.resourcePermissions.list(
          1,
          100,
          debouncedSearch
            ? { filters: { ResourceType: { type: "contains", from: debouncedSearch } } }
            : undefined,
        ),
        api.users.list(1, 100),
        api.sshConnections.list(1, 100),
      ])
      setGrants(grantPage.items)
      setUsers(userPage.items)
      setConnections(connectionPage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("resourceAccess.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleRevoke(grant: ResourcePermissionDto) {
    const username = usersByID.get(grant.userId)?.username ?? `#${grant.userId}`
    const ok = await confirm({
      description: t("resourceAccess.confirmRevoke", {
        username,
        level: t(LEVEL_KEYS[grant.level]),
        resourceType: grant.resourceType,
        resourceId: grant.resourceId,
      }),
    })
    if (!ok) return
    try {
      await api.resourcePermissions.revoke(grant.id)
      toast.success(t("resourceAccess.revoked"))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("resourceAccess.revokeFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("resourceAccess.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("resourceAccess.subtitle")}</p>
        </div>
        <GrantDialog users={users} connections={connections} onGranted={load} />
      </div>

      <SearchInput
        value={search}
        onChange={setSearch}
        placeholder={t("resourceAccess.searchPlaceholder")}
        className="max-w-sm"
      />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("resourceAccess.columnUser")}</TableHead>
              <TableHead>{t("resourceAccess.columnResourceType")}</TableHead>
              <TableHead>{t("resourceAccess.columnResourceId")}</TableHead>
              <TableHead>{t("resourceAccess.columnLevel")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {grants === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={5}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {grants?.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  {debouncedSearch ? t("common.noSearchResults") : t("resourceAccess.empty")}
                </TableCell>
              </TableRow>
            )}

            {grants?.map((grant) => {
              const user = usersByID.get(grant.userId)
              const connection =
                grant.resourceType === RESOURCE_TYPE_SSH_CONNECTION ? connectionsByID.get(grant.resourceId) : undefined
              return (
                <TableRow key={grant.id}>
                  <TableCell className="font-medium">{user?.username ?? `#${grant.userId}`}</TableCell>
                  <TableCell className="text-muted-foreground">{grant.resourceType}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {connection ? `${connection.name} (#${grant.resourceId})` : `#${grant.resourceId}`}
                  </TableCell>
                  <TableCell>
                    <Badge variant={grant.level === "forbidden" ? "secondary" : "default"}>
                      {t(LEVEL_KEYS[grant.level])}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-end">
                    <Button variant="destructive" size="sm" onClick={() => handleRevoke(grant)}>
                      {t("resourceAccess.revoke")}
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function GrantDialog({
  users,
  connections,
  onGranted,
}: {
  users: UserDto[]
  connections: SSHConnectionDto[]
  onGranted: () => void
}) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [userId, setUserId] = useState("")
  const [resourceType, setResourceType] = useState(RESOURCE_TYPE_SSH_CONNECTION)
  const [resourceId, setResourceId] = useState("")
  const [level, setLevel] = useState<AccessLevel>("read")
  const [loading, setLoading] = useState(false)

  const isSSHConnection = resourceType === RESOURCE_TYPE_SSH_CONNECTION

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!userId || !resourceId) return

    setLoading(true)
    try {
      await api.resourcePermissions.grant({
        userId: Number(userId),
        resourceType,
        resourceId: Number(resourceId),
        level,
      })
      toast.success(t("resourceAccess.granted"))
      setOpen(false)
      setUserId("")
      setResourceId("")
      setLevel("read")
      onGranted()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("resourceAccess.grantFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("resourceAccess.newGrant")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("resourceAccess.createDialogTitle")}</DialogTitle>
            <DialogDescription>{t("resourceAccess.createDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label>{t("resourceAccess.userLabel")}</Label>
              <Select value={userId} onValueChange={(value) => setUserId(value ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(value: string) => users.find((u) => String(u.id) === value)?.username ?? t("resourceAccess.chooseUser")}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {users.map((u) => (
                    <SelectItem key={u.id} value={String(u.id)}>
                      {u.username}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t("resourceAccess.resourceTypeLabel")}</Label>
              <Select value={resourceType} onValueChange={(value) => setResourceType(value ?? RESOURCE_TYPE_SSH_CONNECTION)}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={RESOURCE_TYPE_SSH_CONNECTION}>{RESOURCE_TYPE_SSH_CONNECTION}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t("resourceAccess.resourceIdLabel")}</Label>
              {isSSHConnection ? (
                <Select value={resourceId} onValueChange={(value) => setResourceId(value ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue>
                      {(value: string) => {
                        const conn = connections.find((c) => String(c.id) === value)
                        return conn ? `${conn.name} (#${conn.id})` : t("resourceAccess.chooseConnection")
                      }}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {connections.map((c) => (
                      <SelectItem key={c.id} value={String(c.id)}>
                        {c.name} (#{c.id})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  type="number"
                  value={resourceId}
                  onChange={(e) => setResourceId(e.target.value)}
                  required
                />
              )}
            </div>

            <div className="space-y-2">
              <Label>{t("resourceAccess.levelLabel")}</Label>
              <Select value={level} onValueChange={(value) => setLevel((value as AccessLevel) ?? "read")}>
                <SelectTrigger className="w-full">
                  <SelectValue>{(value: string) => t(LEVEL_KEYS[value as AccessLevel])}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(LEVEL_KEYS) as AccessLevel[]).map((lvl) => (
                    <SelectItem key={lvl} value={lvl}>
                      {t(LEVEL_KEYS[lvl])}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !userId || !resourceId}>
              {loading ? t("resourceAccess.granting") : t("resourceAccess.grant")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
