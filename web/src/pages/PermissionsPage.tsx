import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { useConfirm } from "@/components/ConfirmDialog"
import { SearchInput } from "@/components/SearchInput"
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
import { useLanguage } from "@/lib/i18n"
import { useDebouncedValue } from "@/lib/useDebouncedValue"
import { api, ApiError, type PermissionDto } from "@/lib/api"

export default function PermissionsPage() {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()
  const [permissions, setPermissions] = useState<PermissionDto[] | null>(null)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const load = useCallback(async () => {
    try {
      const page = await api.permissions.list(
        1,
        200,
        debouncedSearch ? { filters: { Key: { type: "contains", from: debouncedSearch } } } : undefined,
      )
      setPermissions(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("permissions.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(permission: PermissionDto) {
    const ok = await confirm({ description: t("permissions.confirmDelete", { key: permission.key }) })
    if (!ok) return
    try {
      await api.permissions.remove(permission.id)
      toast.success(t("permissions.deleted", { key: permission.key }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("permissions.deleteFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("permissions.title")}</h2>
          <p className="text-sm text-muted-foreground">
            {t("permissions.subtitlePrefix")}{" "}
            <code className="rounded bg-muted px-1 py-0.5 text-xs">user:create</code>{" "}
            {t("permissions.subtitleSuffix")}
          </p>
        </div>
        <CreatePermissionDialog onCreated={load} />
      </div>

      <SearchInput
        value={search}
        onChange={setSearch}
        placeholder={t("permissions.searchPlaceholder")}
        className="max-w-sm"
      />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("permissions.columnKey")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
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
                  {debouncedSearch ? t("common.noSearchResults") : t("permissions.empty")}
                </TableCell>
              </TableRow>
            )}

            {permissions?.map((permission) => (
              <TableRow key={permission.id}>
                <TableCell className="font-mono text-sm">{permission.key}</TableCell>
                <TableCell className="text-end">
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(permission)}>
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

function CreatePermissionDialog({ onCreated }: { onCreated: () => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.permissions.create(key)
      toast.success(t("permissions.created", { key }))
      setOpen(false)
      setKey("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("permissions.createFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("permissions.newPermission")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("permissions.createDialogTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="permission-key">{t("permissions.key")}</Label>
            <Input
              id="permission-key"
              placeholder={t("permissions.keyPlaceholder")}
              value={key}
              onChange={(e) => setKey(e.target.value)}
              required
              minLength={2}
            />
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
