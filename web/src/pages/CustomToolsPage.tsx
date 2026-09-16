import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useConfirm } from "@/components/ConfirmDialog"
import { TableFilterBar, useTableFilters, type FilterFieldDef } from "@/components/TableFilters"
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
import { useAccess } from "@/lib/access"
import { useLanguage } from "@/lib/i18n"
import {
  api,
  ApiError,
  type CustomToolDto,
  type CustomToolVersionDto,
  type CustomToolVisibility,
} from "@/lib/api"

export default function CustomToolsPage() {
  const { t } = useLanguage()
  const { hasLevel } = useAccess()
  const { confirm, confirmDialog } = useConfirm()
  const [tools, setTools] = useState<CustomToolDto[] | null>(null)

  const filterDefs = useMemo<FilterFieldDef[]>(
    () => [
      { key: "Name", label: t("customTools.searchPlaceholder"), kind: "search" },
      {
        key: "Visibility",
        label: t("customTools.columnVisibility"),
        kind: "select",
        options: [
          { value: "public", label: t("customTools.visibilityPublic") },
          { value: "private", label: t("customTools.visibilityPrivate") },
        ],
      },
    ],
    [t],
  )
  const { state: filterState, setValue: setFilterValue, filter, hasActiveFilters, reset: resetFilters } =
    useTableFilters(filterDefs)

  const load = useCallback(async () => {
    try {
      const page = await api.customTools.list(1, 100, filter)
      setTools(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("customTools.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter])

  useEffect(() => {
    load()
  }, [load])

  async function handleDelete(tool: CustomToolDto) {
    const ok = await confirm({ description: t("customTools.confirmDelete", { name: tool.name }) })
    if (!ok) return
    try {
      await api.customTools.remove(tool.id)
      toast.success(t("customTools.deleted", { name: tool.name }))
      load()
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        load()
        return
      }
      toast.error(err instanceof ApiError ? err.message : t("customTools.deleteFailed"))
    }
  }

  return (
    <div className="space-y-6">
      {confirmDialog}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">{t("customTools.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("customTools.subtitle")}</p>
        </div>
        {hasLevel("custom_tool", "write") && <CreateToolDialog onCreated={load} />}
      </div>

      <TableFilterBar
        defs={filterDefs}
        state={filterState}
        onChange={setFilterValue}
        onReset={resetFilters}
        hasActiveFilters={hasActiveFilters}
        clearLabel={t("common.clearFilters")}
        allLabel={t("common.allFilter")}
      />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("customTools.columnName")}</TableHead>
              <TableHead>{t("customTools.columnDescription")}</TableHead>
              <TableHead>{t("customTools.columnVisibility")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tools === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={4}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {tools?.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="text-center text-muted-foreground">
                  {hasActiveFilters ? t("common.noSearchResults") : t("customTools.empty")}
                </TableCell>
              </TableRow>
            )}

            {tools?.map((tool) => (
              <TableRow key={tool.id}>
                <TableCell className="font-medium">{tool.name}</TableCell>
                <TableCell className="max-w-md text-muted-foreground">{tool.description}</TableCell>
                <TableCell>
                  <Badge variant={tool.visibility === "public" ? "default" : "secondary"}>
                    {tool.visibility === "public" ? t("customTools.visibilityPublic") : t("customTools.visibilityPrivate")}
                  </Badge>
                </TableCell>
                <TableCell className="text-end space-x-2 rtl:space-x-reverse">
                  <VersionsDialog tool={tool} />
                  {hasLevel("custom_tool", "manage") && (
                    <Button variant="destructive" size="sm" onClick={() => handleDelete(tool)}>
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

function CreateToolDialog({ onCreated }: { onCreated: () => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [paramsSchema, setParamsSchema] = useState('{"type":"object","properties":{}}')
  const [visibility, setVisibility] = useState<CustomToolVisibility>("private")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    try {
      JSON.parse(paramsSchema)
    } catch {
      toast.error(t("customTools.invalidParamsSchema"))
      return
    }

    setLoading(true)
    try {
      await api.customTools.create({ name, description, paramsSchema, visibility })
      toast.success(t("customTools.created", { name }))
      setOpen(false)
      setName("")
      setDescription("")
      setParamsSchema('{"type":"object","properties":{}}')
      setVisibility("private")
      onCreated()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("customTools.createFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button>{t("customTools.newTool")}</Button>} />
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("customTools.createDialogTitle")}</DialogTitle>
            <DialogDescription>{t("customTools.createDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="tool-name">{t("customTools.nameLabel")}</Label>
              <Input
                id="tool-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("customTools.namePlaceholder")}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tool-description">{t("customTools.descriptionLabel")}</Label>
              <Input
                id="tool-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="tool-params-schema">{t("customTools.paramsSchemaLabel")}</Label>
              <textarea
                id="tool-params-schema"
                value={paramsSchema}
                onChange={(e) => setParamsSchema(e.target.value)}
                rows={4}
                spellCheck={false}
                required
                className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
              />
              <p className="text-xs text-muted-foreground">{t("customTools.paramsSchemaHint")}</p>
            </div>
            <div className="space-y-2">
              <Label>{t("customTools.columnVisibility")}</Label>
              <Select value={visibility} onValueChange={(v) => setVisibility((v as CustomToolVisibility) ?? "private")}>
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(v: string) => (v === "public" ? t("customTools.visibilityPublic") : t("customTools.visibilityPrivate"))}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="private">{t("customTools.visibilityPrivate")}</SelectItem>
                  <SelectItem value="public">{t("customTools.visibilityPublic")}</SelectItem>
                </SelectContent>
              </Select>
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

function VersionsDialog({ tool }: { tool: CustomToolDto }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [versions, setVersions] = useState<CustomToolVersionDto[] | null>(null)

  const load = useCallback(async () => {
    try {
      const page = await api.customTools.listVersions(tool.id, 1, 50)
      setVersions(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("customTools.loadVersionsFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tool.id])

  useEffect(() => {
    if (open) load()
  }, [open, load])

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm">{t("customTools.versions")}</Button>} />
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("customTools.versionsDialogTitle", { name: tool.name })}</DialogTitle>
          <DialogDescription>{t("customTools.versionsDialogDescription")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="max-h-64 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("customTools.columnVersion")}</TableHead>
                  <TableHead>{t("customTools.columnSourcePreview")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {versions === null &&
                  Array.from({ length: 2 }).map((_, i) => (
                    <TableRow key={i}>
                      <TableCell colSpan={2}>
                        <Skeleton className="h-6 w-full" />
                      </TableCell>
                    </TableRow>
                  ))}

                {versions?.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={2} className="text-center text-muted-foreground">
                      {t("customTools.noVersions")}
                    </TableCell>
                  </TableRow>
                )}

                {versions?.map((v) => (
                  <TableRow key={v.id}>
                    <TableCell className="font-mono text-sm font-medium">{v.version}</TableCell>
                    <TableCell className="max-w-sm truncate font-mono text-xs text-muted-foreground">
                      {v.sourceCode.slice(0, 80)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <AddVersionForm toolId={tool.id} onAdded={load} />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {t("commandRules.close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function AddVersionForm({ toolId, onAdded }: { toolId: number; onAdded: () => void }) {
  const { t } = useLanguage()
  const [version, setVersion] = useState("")
  const [sourceCode, setSourceCode] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      await api.customTools.createVersion(toolId, { version: version.trim(), sourceCode })
      toast.success(t("customTools.versionAdded"))
      setVersion("")
      setSourceCode("")
      onAdded()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("customTools.addVersionFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-2">
      <div className="grid grid-cols-[1fr_auto] gap-2">
        <div className="space-y-2">
          <Label htmlFor="new-version" className="sr-only">
            {t("customTools.columnVersion")}
          </Label>
          <Input
            id="new-version"
            value={version}
            onChange={(e) => setVersion(e.target.value)}
            placeholder={t("customTools.versionPlaceholder")}
            required
          />
        </div>
        <Button type="submit" disabled={loading || !version.trim() || !sourceCode.trim()}>
          {loading ? t("customTools.adding") : t("customTools.addVersion")}
        </Button>
      </div>
      <textarea
        value={sourceCode}
        onChange={(e) => setSourceCode(e.target.value)}
        placeholder={t("customTools.sourceCodePlaceholder")}
        rows={8}
        spellCheck={false}
        required
        className="w-full min-w-0 rounded-lg border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
      />
    </form>
  )
}
