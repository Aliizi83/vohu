import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
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
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useAccess } from "@/lib/access"
import { useLanguage } from "@/lib/i18n"
import { useDebouncedValue } from "@/lib/useDebouncedValue"
import { api, ApiError, type SSHConnectionDto } from "@/lib/api"

export default function SSHConnectionsPage() {
  const { t } = useLanguage()
  const { hasLevel } = useAccess()
  const { confirm, confirmDialog } = useConfirm()
  const [connections, setConnections] = useState<SSHConnectionDto[] | null>(null)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const load = useCallback(async () => {
    try {
      const page = await api.sshConnections.list(
        1,
        100,
        debouncedSearch ? { filters: { Name: { type: "contains", from: debouncedSearch } } } : undefined,
      )
      setConnections(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("sshConnections.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(conn: SSHConnectionDto) {
    const ok = await confirm({ description: t("sshConnections.confirmDelete", { name: conn.name }) })
    if (!ok) return
    try {
      await api.sshConnections.remove(conn.id)
      toast.success(t("sshConnections.deleted", { name: conn.name }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("sshConnections.deleteFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("sshConnections.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("sshConnections.subtitle")}</p>
        </div>
        {hasLevel("ssh_connection", "write") && <CreateConnectionDialog onCreated={load} />}
      </div>

      <SearchInput
        value={search}
        onChange={setSearch}
        placeholder={t("sshConnections.searchPlaceholder")}
        className="max-w-sm"
      />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("sshConnections.columnId")}</TableHead>
              <TableHead>{t("sshConnections.columnName")}</TableHead>
              <TableHead>{t("sshConnections.columnHost")}</TableHead>
              <TableHead>{t("sshConnections.columnUsername")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {connections === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={5}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {connections?.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  {debouncedSearch ? t("common.noSearchResults") : t("sshConnections.empty")}
                </TableCell>
              </TableRow>
            )}

            {connections?.map((conn) => (
              <TableRow key={conn.id}>
                <TableCell className="text-muted-foreground">#{conn.id}</TableCell>
                <TableCell className="font-medium">{conn.name}</TableCell>
                <TableCell className="text-muted-foreground">
                  {conn.host}:{conn.port}
                </TableCell>
                <TableCell>{conn.username}</TableCell>
                <TableCell className="text-end">
                  {hasLevel("ssh_connection", "manage") && (
                    <Button variant="destructive" size="sm" onClick={() => handleDelete(conn)}>
                      {t("common.delete")}
                    </Button>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function CreateConnectionDialog({ onCreated }: { onCreated: () => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [host, setHost] = useState("")
  const [port, setPort] = useState("22")
  const [username, setUsername] = useState("")
  const [privateKey, setPrivateKey] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.sshConnections.create({
        name,
        host,
        port: Number(port) || undefined,
        username,
        privateKey,
      })
      toast.success(t("sshConnections.created", { name }))
      setOpen(false)
      setName("")
      setHost("")
      setPort("22")
      setUsername("")
      setPrivateKey("")
      onCreated()
    } catch (err) {
      // A failed connection test comes back as a plain error message from
      // the server (e.g. "ssh connection test failed: dial: ..."), the
      // same path as any other ApiError — surfaced here rather than a
      // generic fallback so the user knows *why* it didn't save.
      toast.error(err instanceof ApiError ? err.message : t("sshConnections.createFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("sshConnections.newConnection")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("sshConnections.createDialogTitle")}</DialogTitle>
            <DialogDescription>{t("sshConnections.createDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="conn-name">{t("sshConnections.name")}</Label>
              <Input id="conn-name" value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2 space-y-2">
                <Label htmlFor="conn-host">{t("sshConnections.host")}</Label>
                <Input id="conn-host" value={host} onChange={(e) => setHost(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="conn-port">{t("sshConnections.port")}</Label>
                <Input
                  id="conn-port"
                  type="number"
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="conn-username">{t("sshConnections.username")}</Label>
              <Input
                id="conn-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="conn-private-key">{t("sshConnections.privateKey")}</Label>
              <textarea
                id="conn-private-key"
                value={privateKey}
                onChange={(e) => setPrivateKey(e.target.value)}
                placeholder={t("sshConnections.privateKeyPlaceholder")}
                required
                rows={6}
                spellCheck={false}
                className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading}>
              {loading ? t("sshConnections.testingConnection") : t("common.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
