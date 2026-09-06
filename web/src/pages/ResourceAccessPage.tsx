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
  type GranteeType,
  type ResourceAccessDto,
  type ResourceEffect,
  type RoleDto,
  type SSHConnectionDto,
  type UserDto,
} from "@/lib/api"

// The free-string resource types the backend currently knows about
// (rbac.KnownResourceTypes) — kept in sync by hand since the frontend
// never imports Go code.
const RESOURCE_TYPES = ["user", "role", "ssh_connection", "provider_key", "conversation", "resource_access"] as const

const RESOURCE_TYPE_SSH_CONNECTION = "ssh_connection"

const LEVEL_KEYS: Record<AccessLevel, string> = {
  read: "resourceAccess.levelRead",
  write: "resourceAccess.levelWrite",
  manage: "resourceAccess.levelManage",
}

const EFFECT_KEYS: Record<ResourceEffect, string> = {
  accepted: "resourceAccess.effectAccepted",
  prohibited: "resourceAccess.effectProhibited",
}

export default function ResourceAccessPage() {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()
  const [grants, setGrants] = useState<ResourceAccessDto[] | null>(null)
  const [users, setUsers] = useState<UserDto[]>([])
  const [roles, setRoles] = useState<RoleDto[]>([])
  const [connections, setConnections] = useState<SSHConnectionDto[]>([])
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const usersByID = new Map(users.map((u) => [u.id, u]))
  const rolesByID = new Map(roles.map((r) => [r.id, r]))
  const connectionsByID = new Map(connections.map((c) => [c.id, c]))

  const load = useCallback(async () => {
    try {
      const [grantPage, userPage, rolePage, connectionPage] = await Promise.all([
        api.resourceAccess.list(
          1,
          100,
          debouncedSearch
            ? { filters: { ResourceType: { type: "contains", from: debouncedSearch } } }
            : undefined,
        ),
        api.users.list(1, 100),
        api.roles.list(1, 100),
        api.sshConnections.list(1, 100),
      ])
      setGrants(grantPage.items)
      setUsers(userPage.items)
      setRoles(rolePage.items)
      setConnections(connectionPage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("resourceAccess.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  function granteeLabel(grant: ResourceAccessDto) {
    if (grant.granteeType === "role") {
      return rolesByID.get(grant.granteeId)?.name ?? `role #${grant.granteeId}`
    }
    return usersByID.get(grant.granteeId)?.username ?? `user #${grant.granteeId}`
  }

  async function handleRevoke(grant: ResourceAccessDto) {
    const ok = await confirm({
      description: t("resourceAccess.confirmRevoke", {
        grantee: granteeLabel(grant),
        level: t(LEVEL_KEYS[grant.level]),
        resourceType: grant.resourceType,
        resourceId: grant.resourceId,
      }),
    })
    if (!ok) return
    try {
      await api.resourceAccess.revoke(grant.id)
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
        <GrantDialog users={users} roles={roles} connections={connections} onGranted={load} />
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
              <TableHead>{t("resourceAccess.columnGrantee")}</TableHead>
              <TableHead>{t("resourceAccess.columnResourceType")}</TableHead>
              <TableHead>{t("resourceAccess.columnResourceId")}</TableHead>
              <TableHead>{t("resourceAccess.columnLevel")}</TableHead>
              <TableHead>{t("resourceAccess.columnEffect")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {grants === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={6}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {grants?.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  {debouncedSearch ? t("common.noSearchResults") : t("resourceAccess.empty")}
                </TableCell>
              </TableRow>
            )}

            {grants?.map((grant) => {
              const connection =
                grant.resourceType === RESOURCE_TYPE_SSH_CONNECTION ? connectionsByID.get(grant.resourceId) : undefined
              return (
                <TableRow key={grant.id}>
                  <TableCell className="font-medium">
                    {granteeLabel(grant)}
                    <span className="ms-1 text-xs text-muted-foreground">
                      ({t(grant.granteeType === "role" ? "resourceAccess.granteeRole" : "resourceAccess.granteeUser")})
                    </span>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{grant.resourceType}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {connection ? `${connection.name} (#${grant.resourceId})` : `#${grant.resourceId}`}
                  </TableCell>
                  <TableCell>
                    <Badge>{t(LEVEL_KEYS[grant.level])}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant={grant.effect === "prohibited" ? "destructive" : "secondary"}>
                      {t(EFFECT_KEYS[grant.effect])}
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
  roles,
  connections,
  onGranted,
}: {
  users: UserDto[]
  roles: RoleDto[]
  connections: SSHConnectionDto[]
  onGranted: () => void
}) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [granteeType, setGranteeType] = useState<GranteeType>("user")
  const [granteeId, setGranteeId] = useState("")
  const [resourceType, setResourceType] = useState<string>(RESOURCE_TYPE_SSH_CONNECTION)
  const [resourceId, setResourceId] = useState("")
  const [level, setLevel] = useState<AccessLevel>("read")
  const [effect, setEffect] = useState<ResourceEffect>("accepted")
  const [loading, setLoading] = useState(false)

  const isSSHConnection = resourceType === RESOURCE_TYPE_SSH_CONNECTION

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!granteeId || !resourceId) return

    setLoading(true)
    try {
      await api.resourceAccess.grant({
        granteeType,
        granteeId: Number(granteeId),
        resourceType,
        resourceId: Number(resourceId),
        level,
        effect,
      })
      toast.success(t("resourceAccess.granted"))
      setOpen(false)
      setGranteeId("")
      setResourceId("")
      setLevel("read")
      setEffect("accepted")
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
              <Label>{t("resourceAccess.granteeTypeLabel")}</Label>
              <Select
                value={granteeType}
                onValueChange={(value) => {
                  setGranteeType((value as GranteeType) ?? "user")
                  setGranteeId("")
                }}
              >
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(value: string) =>
                      t(value === "role" ? "resourceAccess.granteeRole" : "resourceAccess.granteeUser")
                    }
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">{t("resourceAccess.granteeUser")}</SelectItem>
                  <SelectItem value="role">{t("resourceAccess.granteeRole")}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>{t("resourceAccess.granteeLabel")}</Label>
              {granteeType === "role" ? (
                <Select value={granteeId} onValueChange={(value) => setGranteeId(value ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue>
                      {(value: string) => roles.find((r) => String(r.id) === value)?.name ?? t("resourceAccess.chooseRole")}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {roles.map((r) => (
                      <SelectItem key={r.id} value={String(r.id)}>
                        {r.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Select value={granteeId} onValueChange={(value) => setGranteeId(value ?? "")}>
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
              )}
            </div>

            <div className="space-y-2">
              <Label>{t("resourceAccess.resourceTypeLabel")}</Label>
              <Select
                value={resourceType}
                onValueChange={(value) => {
                  setResourceType(value ?? RESOURCE_TYPE_SSH_CONNECTION)
                  setResourceId("")
                }}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {RESOURCE_TYPES.map((rt) => (
                    <SelectItem key={rt} value={rt}>
                      {rt}
                    </SelectItem>
                  ))}
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
                  placeholder={t("resourceAccess.resourceIdWildcardHint")}
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

            <div className="space-y-2">
              <Label>{t("resourceAccess.effectLabel")}</Label>
              <Select value={effect} onValueChange={(value) => setEffect((value as ResourceEffect) ?? "accepted")}>
                <SelectTrigger className="w-full">
                  <SelectValue>{(value: string) => t(EFFECT_KEYS[value as ResourceEffect])}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {(Object.keys(EFFECT_KEYS) as ResourceEffect[]).map((eff) => (
                    <SelectItem key={eff} value={eff}>
                      {t(EFFECT_KEYS[eff])}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !granteeId || !resourceId}>
              {loading ? t("resourceAccess.granting") : t("resourceAccess.grant")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
