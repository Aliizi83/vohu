import { useCallback, useEffect, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { api, ApiError, type LLMProvider, type ProviderKeyDto } from "@/lib/api"

const PROVIDERS: { value: LLMProvider; label: string }[] = [
  { value: "gemini", label: "Gemini" },
  { value: "anthropic", label: "Anthropic" },
  { value: "openai", label: "OpenAI-compatible" },
]

export default function ApiKeysPage() {
  const [myKeys, setMyKeys] = useState<ProviderKeyDto[] | null>(null)
  const [globalKeys, setGlobalKeys] = useState<ProviderKeyDto[] | null>(null)
  // null = still loading / unknown, false = the caller isn't allowed to
  // manage global keys at all (403), so that section is hidden entirely
  // rather than shown broken.
  const [canManageGlobal, setCanManageGlobal] = useState<boolean | null>(null)

  const loadMine = useCallback(async () => {
    try {
      setMyKeys(await api.providerKeys.listMine())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load your API keys")
    }
  }, [])

  const loadGlobal = useCallback(async () => {
    try {
      setGlobalKeys(await api.providerKeys.listGlobal())
      setCanManageGlobal(true)
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setCanManageGlobal(false)
        return
      }
      toast.error(err instanceof ApiError ? err.message : "Failed to load global API keys")
    }
  }, [])

  useEffect(() => {
    loadMine()
    loadGlobal()
  }, [loadMine, loadGlobal])

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-semibold">API Keys</h2>
        <p className="text-sm text-muted-foreground">
          Credentials for the LLM providers the chat agent uses. A personal key always
          takes priority over the global default; if you don't set one, your chats use
          whatever an admin configured for everyone.
        </p>
      </div>

      <ProviderKeyTable
        title="My keys"
        description="Only you can see or use these — never shared with other users."
        keys={myKeys}
        onSet={async (data) => {
          await api.providerKeys.setMine(data)
          await loadMine()
        }}
        onRemove={async (provider) => {
          await api.providerKeys.removeMine(provider)
          await loadMine()
        }}
      />

      {canManageGlobal && (
        <ProviderKeyTable
          title="Global default keys"
          description="Used for any user who hasn't set their own personal key for that provider."
          keys={globalKeys}
          onSet={async (data) => {
            await api.providerKeys.setGlobal(data)
            await loadGlobal()
          }}
          onRemove={async (provider) => {
            await api.providerKeys.removeGlobal(provider)
            await loadGlobal()
          }}
        />
      )}
    </div>
  )
}

function ProviderKeyTable({
  title,
  description,
  keys,
  onSet,
  onRemove,
}: {
  title: string
  description: string
  keys: ProviderKeyDto[] | null
  onSet: (data: { provider: LLMProvider; apiKey: string; baseUrl?: string; workspaceId?: string }) => Promise<void>
  onRemove: (provider: LLMProvider) => Promise<void>
}) {
  async function handleRemove(provider: LLMProvider) {
    if (!confirm(`Remove the ${provider} key? Chats using it will fall back to any other configured key.`)) return
    try {
      await onRemove(provider)
      toast.success(`${provider} key removed`)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to remove key")
    }
  }

  const configured = new Map(keys?.map((k) => [k.provider, k]) ?? [])

  return (
    <div className="space-y-3">
      <div>
        <h3 className="font-medium">{title}</h3>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Provider</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Base URL / Workspace</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {PROVIDERS.map(({ value, label }) => {
              const key = configured.get(value)
              return (
                <TableRow key={value}>
                  <TableCell className="font-medium">{label}</TableCell>
                  <TableCell>
                    {keys === null ? (
                      <span className="text-muted-foreground">…</span>
                    ) : key ? (
                      <Badge>Configured</Badge>
                    ) : (
                      <Badge variant="secondary">Not set</Badge>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {key?.baseUrl || key?.workspaceId
                      ? [key.baseUrl, key.workspaceId].filter(Boolean).join(" · ")
                      : "—"}
                  </TableCell>
                  <TableCell className="flex justify-end gap-2">
                    <SetKeyDialog provider={value} label={label} onSet={onSet} />
                    {key && (
                      <Button variant="destructive" size="sm" onClick={() => handleRemove(value)}>
                        Remove
                      </Button>
                    )}
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

function SetKeyDialog({
  provider,
  label,
  onSet,
}: {
  provider: LLMProvider
  label: string
  onSet: (data: { provider: LLMProvider; apiKey: string; baseUrl?: string; workspaceId?: string }) => Promise<void>
}) {
  const [open, setOpen] = useState(false)
  const [apiKey, setApiKey] = useState("")
  const [baseUrl, setBaseUrl] = useState("")
  const [workspaceId, setWorkspaceId] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await onSet({
        provider,
        apiKey,
        baseUrl: baseUrl || undefined,
        workspaceId: workspaceId || undefined,
      })
      toast.success(`${label} key saved`)
      setOpen(false)
      setApiKey("")
      setBaseUrl("")
      setWorkspaceId("")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save key")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            Set key
          </Button>
        }
      />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Set {label} key</DialogTitle>
            <DialogDescription>
              The key is encrypted before it's stored and is never returned by the API
              again.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor={`${provider}-api-key`}>API key</Label>
              <Input
                id={`${provider}-api-key`}
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                required
              />
            </div>
            {provider === "openai" && (
              <div className="space-y-2">
                <Label htmlFor={`${provider}-base-url`}>Base URL (optional)</Label>
                <Input
                  id={`${provider}-base-url`}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder="Leave empty for OpenAI itself"
                />
              </div>
            )}
            {provider === "anthropic" && (
              <div className="space-y-2">
                <Label htmlFor={`${provider}-workspace-id`}>Workspace ID (optional)</Label>
                <Input
                  id={`${provider}-workspace-id`}
                  value={workspaceId}
                  onChange={(e) => setWorkspaceId(e.target.value)}
                  placeholder="Only needed for a multi-workspace organization"
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !apiKey}>
              {loading ? "Saving..." : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
