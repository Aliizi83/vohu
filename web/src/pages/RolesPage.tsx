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
import { api, ApiError, type RoleDto } from "@/lib/api"

export default function RolesPage() {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()
  const [roles, setRoles] = useState<RoleDto[] | null>(null)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const load = useCallback(async () => {
    try {
      const rolePage = await api.roles.list(
        1,
        50,
        debouncedSearch ? { filters: { Name: { type: "contains", from: debouncedSearch } } } : undefined,
      )
      setRoles(rolePage.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("roles.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(role: RoleDto) {
    const ok = await confirm({ description: t("roles.confirmDelete", { name: role.name }) })
    if (!ok) return
    try {
      await api.roles.remove(role.id)
      toast.success(t("roles.deleted", { name: role.name }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("roles.deleteFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("roles.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("roles.subtitle")}</p>
        </div>
        <CreateRoleDialog onCreated={load} />
      </div>

      <SearchInput value={search} onChange={setSearch} placeholder={t("roles.searchPlaceholder")} className="max-w-sm" />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("roles.columnName")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
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
                  {debouncedSearch ? t("common.noSearchResults") : t("roles.empty")}
                </TableCell>
              </TableRow>
            )}

            {roles?.map((role) => (
              <TableRow key={role.id}>
                <TableCell className="font-medium">{role.name}</TableCell>
                <TableCell className="flex justify-end gap-2">
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(role)}>
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

function CreateRoleDialog({ onCreated }: { onCreated: () => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.roles.create(name)
      toast.success(t("roles.created", { name }))
      setOpen(false)
      setName("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("roles.createFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("roles.newRole")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("roles.createDialogTitle")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-2 py-4">
            <Label htmlFor="role-name">{t("roles.name")}</Label>
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
              {loading ? t("common.creating") : t("common.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
