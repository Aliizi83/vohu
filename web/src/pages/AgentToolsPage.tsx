import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { SearchInput } from "@/components/SearchInput"
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
import { api, ApiError, type AgentToolDto } from "@/lib/api"

export default function AgentToolsPage() {
  const { t } = useLanguage()
  const { hasLevel } = useAccess()
  const [agentTools, setAgentTools] = useState<AgentToolDto[] | null>(null)
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search)

  const load = useCallback(async () => {
    try {
      const page = await api.agentTools.list(
        1,
        100,
        debouncedSearch ? { filters: { Name: { type: "contains", from: debouncedSearch } } } : undefined,
      )
      setAgentTools(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("agentTools.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  useEffect(() => {
    load()
  }, [load])

  async function handleToggleVisibility(tool: AgentToolDto) {
    const next = tool.visibility === "public" ? "private" : "public"
    try {
      await api.agentTools.setVisibility(tool.id, next)
      toast.success(t("agentTools.visibilityChanged", { name: tool.name }))
      load()
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("agentTools.updateFailed"))
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-semibold">{t("agentTools.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("agentTools.subtitle")}</p>
      </div>

      <SearchInput
        value={search}
        onChange={setSearch}
        placeholder={t("agentTools.searchPlaceholder")}
        className="max-w-sm"
      />

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("agentTools.columnName")}</TableHead>
              <TableHead>{t("agentTools.columnDescription")}</TableHead>
              <TableHead>{t("agentTools.columnStatus")}</TableHead>
              <TableHead>{t("agentTools.columnVisibility")}</TableHead>
              <TableHead className="text-end">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {agentTools === null &&
              Array.from({ length: 3 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={5}>
                    <Skeleton className="h-6 w-full" />
                  </TableCell>
                </TableRow>
              ))}

            {agentTools?.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-muted-foreground">
                  {debouncedSearch ? t("common.noSearchResults") : t("agentTools.empty")}
                </TableCell>
              </TableRow>
            )}

            {agentTools?.map((tool) => (
              <TableRow key={tool.id}>
                <TableCell className="font-medium">{tool.name}</TableCell>
                <TableCell className="max-w-md text-muted-foreground">{tool.description}</TableCell>
                <TableCell>
                  <Badge variant={tool.implemented ? "default" : "secondary"}>
                    {tool.implemented ? t("agentTools.implemented") : t("agentTools.notImplemented")}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Badge variant={tool.visibility === "public" ? "default" : "secondary"}>
                    {tool.visibility === "public" ? t("agentTools.visibilityPublic") : t("agentTools.visibilityPrivate")}
                  </Badge>
                </TableCell>
                <TableCell className="text-end">
                  {hasLevel("agent_tool", "manage") && (
                    <Button variant="outline" size="sm" onClick={() => handleToggleVisibility(tool)}>
                      {tool.visibility === "public" ? t("agentTools.makePrivate") : t("agentTools.makePublic")}
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
