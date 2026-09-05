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
import { api, ApiError, type SSHConnectionDto } from "@/lib/api"

export default function SSHConnectionsPage() {
  const [connections, setConnections] = useState<SSHConnectionDto[] | null>(null)

  const load = useCallback(async () => {
    try {
      const page = await api.sshConnections.list(1, 100)
      setConnections(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load SSH connections")
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(conn: SSHConnectionDto) {
    if (!confirm(`Delete connection "${conn.name}"? This can't be undone.`)) return
    try {
      await api.sshConnections.remove(conn.id)
      toast.success(`${conn.name} deleted`)
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Delete failed")
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">SSH Connections</h2>
          <p className="text-sm text-muted-foreground">
            Servers the agent can reach over SSH. Only you (and anyone else explicitly
            granted access) can use a connection you create — see Roles/Permissions for
            who may manage connections at all.
          </p>
        </div>
        <CreateConnectionDialog onCreated={load} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Host</TableHead>
              <TableHead>Username</TableHead>
              <TableHead>Auth</TableHead>
              <TableHead className="text-right">Actions</TableHead>
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
                  No SSH connections yet.
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
                <TableCell className="text-muted-foreground">{conn.authMethod}</TableCell>
                <TableCell className="text-right">
                  <Button variant="destructive" size="sm" onClick={() => handleDelete(conn)}>
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

function CreateConnectionDialog({ onCreated }: { onCreated: () => void }) {
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
      toast.success(`Connection "${name}" created`)
      setOpen(false)
      setName("")
      setHost("")
      setPort("22")
      setUsername("")
      setAuthMethod("password")
      setSecret("")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create connection")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>New connection</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>New SSH connection</DialogTitle>
            <DialogDescription>
              The password/key is encrypted before it's stored and is never returned by
              the API again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="conn-name">Name</Label>
              <Input id="conn-name" value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2 space-y-2">
                <Label htmlFor="conn-host">Host</Label>
                <Input id="conn-host" value={host} onChange={(e) => setHost(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="conn-port">Port</Label>
                <Input
                  id="conn-port"
                  type="number"
                  value={port}
                  onChange={(e) => setPort(e.target.value)}
                />
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="conn-username">Username</Label>
              <Input
                id="conn-username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label>Auth method</Label>
              <Select
                value={authMethod}
                onValueChange={(value) => setAuthMethod((value as "password" | "private_key") ?? "password")}
              >
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(value: string) => (value === "password" ? "Password" : "Private key")}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="password">Password</SelectItem>
                  <SelectItem value="private_key">Private key</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="conn-secret">
                {authMethod === "password" ? "Password" : "Private key (PEM)"}
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
              {loading ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
