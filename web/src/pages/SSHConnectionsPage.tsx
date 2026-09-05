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
import { useLanguage } from "@/lib/i18n"
import { api, ApiError, type SSHConnectionDto } from "@/lib/api"

export default function SSHConnectionsPage() {
  const { t } = useLanguage()
  const [connections, setConnections] = useState<SSHConnectionDto[] | null>(null)

  const load = useCallback(async () => {
    try {
      const page = await api.sshConnections.list(1, 100)
      setConnections(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("sshConnections.loadFailed"))
    }
  }, [t])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(conn: SSHConnectionDto) {
    if (!confirm(t("sshConnections.confirmDelete", { name: conn.name }))) return
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
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("sshConnections.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("sshConnections.subtitle")}</p>
        </div>
        <CreateConnectionDialog onCreated={load} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("sshConnections.columnId")}</TableHead>
              <TableHead>{t("sshConnections.columnName")}</TableHead>
              <TableHead>{t("sshConnections.columnHost")}</TableHead>
              <TableHead>{t("sshConnections.columnUsername")}</TableHead>
              <TableHead>{t("sshConnections.columnAuth")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {connections === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={6}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {connections?.length === 0 && (
              <TableRow>
                <TableCell colSpan={6} className="text-center text-muted-foreground">
                  {t("sshConnections.empty")}
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
                <TableCell className="text-muted-foreground">
                  {conn.authMethod === "password" ? t("sshConnections.authPassword") : t("sshConnections.authPrivateKey")}
                </TableCell>
                <TableCell className="text-end">
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(conn)}>
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

function CreateConnectionDialog({ onCreated }: { onCreated: () => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [host, setHost] = useState("")
  const [port, setPort] = useState("22")
  const [username, setUsername] = useState("")
  const [authMethod, setAuthMethod] = useState<"password" | "private_key">("password")
  const [secret, setSecret] = useState("")
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
        authMethod,
        secret,
      })
      toast.success(t("sshConnections.created", { name }))
      setOpen(false)
      setName("")
      setHost("")
      setPort("22")
      setUsername("")
      setAuthMethod("password")
      setSecret("")
      onCreated()
    } catch (err) {
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
              <Label>{t("sshConnections.authMethod")}</Label>
              <Select
                value={authMethod}
                onValueChange={(value) => setAuthMethod((value as "password" | "private_key") ?? "password")}
              >
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(value: string) =>
                      value === "password" ? t("sshConnections.authPassword") : t("sshConnections.authPrivateKey")
                    }
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="password">{t("sshConnections.authPassword")}</SelectItem>
                  <SelectItem value="private_key">{t("sshConnections.authPrivateKey")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="conn-secret">
                {authMethod === "password" ? t("sshConnections.secretPassword") : t("sshConnections.secretPrivateKey")}
              </Label>
              <Input
                id="conn-secret"
                type={authMethod === "password" ? "password" : "text"}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                required
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
