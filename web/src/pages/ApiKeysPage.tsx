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
import { useConfirm } from "@/components/ConfirmDialog"
import { useLanguage } from "@/lib/i18n"
import { api, ApiError, type CustomModelDto, type LLMProvider, type ProviderKeyDto } from "@/lib/api"

const PROVIDERS: { value: LLMProvider; label: string }[] = [
  { value: "gemini", label: "Gemini" },
  { value: "anthropic", label: "Anthropic" },
  { value: "openai", label: "OpenAI-compatible" },
]

export default function ApiKeysPage() {
  const { t } = useLanguage()
  const [myKeys, setMyKeys] = useState<ProviderKeyDto[] | null>(null)
  const [globalKeys, setGlobalKeys] = useState<ProviderKeyDto[] | null>(null)
  // null = still loading / unknown, false = the caller isn't allowed to
  // manage global keys at all (403), so that section is hidden entirely
  // rather than shown broken.
  const [canManageGlobal, setCanManageGlobal] = useState<boolean | null>(null)

  const [myCustomModels, setMyCustomModels] = useState<CustomModelDto[] | null>(null)
  const [globalCustomModels, setGlobalCustomModels] = useState<CustomModelDto[] | null>(null)
  const [canManageGlobalCustomModels, setCanManageGlobalCustomModels] = useState<boolean | null>(null)

  const loadMine = useCallback(async () => {
    try {
      setMyKeys(await api.providerKeys.listMine())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.loadMineFailed"))
    }
  }, [t])

  const loadGlobal = useCallback(async () => {
    try {
      setGlobalKeys(await api.providerKeys.listGlobal())
      setCanManageGlobal(true)
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setCanManageGlobal(false)
        return
      }
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.loadGlobalFailed"))
    }
  }, [t])

  const loadMyCustomModels = useCallback(async () => {
    try {
      setMyCustomModels(await api.customModels.listMine())
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.loadMineFailed"))
    }
  }, [t])

  const loadGlobalCustomModels = useCallback(async () => {
    try {
      setGlobalCustomModels(await api.customModels.listGlobal())
      setCanManageGlobalCustomModels(true)
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setCanManageGlobalCustomModels(false)
        return
      }
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.loadGlobalFailed"))
    }
  }, [t])

  useEffect(() => {
    loadMine()
    loadGlobal()
    loadMyCustomModels()
    loadGlobalCustomModels()
  }, [loadMine, loadGlobal, loadMyCustomModels, loadGlobalCustomModels])

  return (
    <div className="space-y-8">
      <div>
        <h2 className="text-2xl font-semibold">{t("apiKeys.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("apiKeys.subtitle")}</p>
      </div>

      <ProviderKeyTable
        title={t("apiKeys.myKeysTitle")}
        description={t("apiKeys.myKeysDescription")}
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
          title={t("apiKeys.globalKeysTitle")}
          description={t("apiKeys.globalKeysDescription")}
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

      <CustomModelSection
        title={t("apiKeys.myCustomModelsTitle")}
        description={t("apiKeys.myCustomModelsDescription")}
        models={myCustomModels}
        suggestions={collectModelNameSuggestions(myCustomModels, globalCustomModels)}
        onCreate={async (data) => {
          await api.customModels.createMine(data)
          await loadMyCustomModels()
        }}
        onRemove={async (id) => {
          await api.customModels.removeMine(id)
          await loadMyCustomModels()
        }}
      />

      {canManageGlobalCustomModels && (
        <CustomModelSection
          title={t("apiKeys.globalCustomModelsTitle")}
          description={t("apiKeys.globalCustomModelsDescription")}
          models={globalCustomModels}
          suggestions={collectModelNameSuggestions(myCustomModels, globalCustomModels)}
          onCreate={async (data) => {
            await api.customModels.createGlobal(data)
            await loadGlobalCustomModels()
          }}
          onRemove={async (id) => {
            await api.customModels.removeGlobal(id)
            await loadGlobalCustomModels()
          }}
        />
      )}
    </div>
  )
}

// collectModelNameSuggestions is the "suggest from the database" feature —
// a plain distinct-values list from whatever presets are already loaded
// (the caller's own plus any visible global ones), fed into the Add
// dialog's <datalist> below. No dedicated backend endpoint: the page
// already has to fetch these lists to render the tables, so there's
// nothing more to ask the server for.
function collectModelNameSuggestions(
  mine: CustomModelDto[] | null,
  global: CustomModelDto[] | null,
): string[] {
  const names = new Set<string>()
  for (const m of mine ?? []) names.add(m.modelName)
  for (const m of global ?? []) names.add(m.modelName)
  return Array.from(names).sort()
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
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()

  async function handleRemove(provider: LLMProvider) {
    const ok = await confirm({ description: t("apiKeys.confirmRemove", { provider }) })
    if (!ok) return
    try {
      await onRemove(provider)
      toast.success(t("apiKeys.removed", { provider }))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.removeFailed"))
    }
  }

  const configured = new Map(keys?.map((k) => [k.provider, k]) ?? [])

  return (
    <div className="space-y-3">
      {confirmDialog}
      <div>
        <h3 className="font-medium">{title}</h3>
        <p className="text-sm text-muted-foreground">{description}</p>
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("apiKeys.columnProvider")}</TableHead>
              <TableHead>{t("apiKeys.columnStatus")}</TableHead>
              <TableHead>{t("apiKeys.columnBaseUrlWorkspace")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
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
                      <Badge>{t("apiKeys.configured")}</Badge>
                    ) : (
                      <Badge variant="secondary">{t("apiKeys.notSet")}</Badge>
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
                        {t("apiKeys.remove")}
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
  const { t } = useLanguage()
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
      toast.success(t("apiKeys.saved", { label }))
      setOpen(false)
      setApiKey("")
      setBaseUrl("")
      setWorkspaceId("")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.saveFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            {t("apiKeys.setKey")}
          </Button>
        }
      />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("apiKeys.setDialogTitle", { label })}</DialogTitle>
            <DialogDescription>{t("apiKeys.setDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor={`${provider}-api-key`}>{t("apiKeys.apiKeyLabel")}</Label>
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
                <Label htmlFor={`${provider}-base-url`}>{t("apiKeys.baseUrlLabel")}</Label>
                <Input
                  id={`${provider}-base-url`}
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  placeholder={t("apiKeys.baseUrlPlaceholder")}
                />
              </div>
            )}
            {provider === "anthropic" && (
              <div className="space-y-2">
                <Label htmlFor={`${provider}-workspace-id`}>{t("apiKeys.workspaceIdLabel")}</Label>
                <Input
                  id={`${provider}-workspace-id`}
                  value={workspaceId}
                  onChange={(e) => setWorkspaceId(e.target.value)}
                  placeholder={t("apiKeys.workspaceIdPlaceholder")}
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !apiKey}>
              {loading ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function CustomModelSection({
  title,
  description,
  models,
  suggestions,
  onCreate,
  onRemove,
}: {
  title: string
  description: string
  models: CustomModelDto[] | null
  suggestions: string[]
  onCreate: (data: { name: string; baseUrl: string; modelName: string; apiKey: string }) => Promise<void>
  onRemove: (id: number) => Promise<void>
}) {
  const { t } = useLanguage()
  const { confirm, confirmDialog } = useConfirm()

  async function handleRemove(model: CustomModelDto) {
    const ok = await confirm({ description: t("apiKeys.confirmRemoveCustomModel", { name: model.name }) })
    if (!ok) return
    try {
      await onRemove(model.id)
      toast.success(t("apiKeys.removedCustomModel", { name: model.name }))
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.removeFailed"))
    }
  }

  return (
    <div className="space-y-3">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h3 className="font-medium">{title}</h3>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
        <AddCustomModelDialog suggestions={suggestions} onCreate={onCreate} />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("apiKeys.columnName")}</TableHead>
              <TableHead>{t("apiKeys.columnModelName")}</TableHead>
              <TableHead>{t("apiKeys.columnBaseUrlWorkspace")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {models === null &&
              Array.from({ length: 2 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={4} className="text-muted-foreground">
                    …
                  </TableCell>
                </TableRow>
              ))}

            {models?.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  {t("apiKeys.noCustomModels")}
                </TableCell>
              </TableRow>
            )}

            {models?.map((model) => (
              <TableRow key={model.id}>
                <TableCell className="font-medium">{model.name}</TableCell>
                <TableCell className="text-muted-foreground">{model.modelName}</TableCell>
                <TableCell className="text-muted-foreground">{model.baseUrl}</TableCell>
                <TableCell className="text-end">
                  <Button variant="destructive" size="sm" onClick={() => handleRemove(model)}>
                    {t("apiKeys.remove")}
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

function AddCustomModelDialog({
  suggestions,
  onCreate,
}: {
  suggestions: string[]
  onCreate: (data: { name: string; baseUrl: string; modelName: string; apiKey: string }) => Promise<void>
}) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [baseUrl, setBaseUrl] = useState("")
  const [modelName, setModelName] = useState("")
  const [apiKey, setApiKey] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await onCreate({ name, baseUrl, modelName, apiKey })
      toast.success(t("apiKeys.customModelAdded", { name }))
      setOpen(false)
      setName("")
      setBaseUrl("")
      setModelName("")
      setApiKey("")
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("apiKeys.addCustomModelFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm">{t("apiKeys.addCustomModel")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("apiKeys.addCustomModelDialogTitle")}</DialogTitle>
            <DialogDescription>{t("apiKeys.addCustomModelDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="custom-model-name">{t("apiKeys.customModelNameLabel")}</Label>
              <Input
                id="custom-model-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("apiKeys.customModelNamePlaceholder")}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="custom-model-base-url">{t("apiKeys.customModelBaseUrlLabel")}</Label>
              <Input
                id="custom-model-base-url"
                value={baseUrl}
                onChange={(e) => setBaseUrl(e.target.value)}
                placeholder={t("apiKeys.customModelBaseUrlPlaceholder")}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="custom-model-model-name">{t("apiKeys.modelNameLabel")}</Label>
              <Input
                id="custom-model-model-name"
                list="custom-model-name-suggestions"
                value={modelName}
                onChange={(e) => setModelName(e.target.value)}
                placeholder={t("apiKeys.modelNamePlaceholder")}
                required
              />
              <datalist id="custom-model-name-suggestions">
                {suggestions.map((s) => (
                  <option key={s} value={s} />
                ))}
              </datalist>
            </div>
            <div className="space-y-2">
              <Label htmlFor="custom-model-api-key">{t("apiKeys.apiKeyLabel")}</Label>
              <Input
                id="custom-model-api-key"
                type="password"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                required
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !name || !baseUrl || !modelName || !apiKey}>
              {loading ? t("common.saving") : t("common.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
