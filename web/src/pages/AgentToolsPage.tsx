import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { TableFilterBar, useTableFilters, type FilterFieldDef } from "@/components/TableFilters"
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
import { api, ApiError, type AgentToolDto } from "@/lib/api"

export default function AgentToolsPage() {
  const { t } = useLanguage()
  const { hasLevelOnResource } = useAccess()
  const [agentTools, setAgentTools] = useState<AgentToolDto[] | null>(null)

  const filterDefs = useMemo<FilterFieldDef[]>(
    () => [
      { key: "Name", label: t("agentTools.searchPlaceholder"), kind: "search" },
      {
        key: "Implemented",
        label: t("agentTools.columnStatus"),
        kind: "select",
        options: [
          { value: "true", label: t("agentTools.implemented") },
          { value: "false", label: t("agentTools.notImplemented") },
        ],
      },
      {
        key: "Visibility",
        label: t("agentTools.columnVisibility"),
        kind: "select",
        options: [
          { value: "public", label: t("agentTools.visibilityPublic") },
          { value: "private", label: t("agentTools.visibilityPrivate") },
        ],
      },
    ],
    [t],
  )
  const { state: filterState, setValue: setFilterValue, filter, hasActiveFilters, reset: resetFilters } =
    useTableFilters(filterDefs)

  const load = useCallback(async () => {
    try {
      const page = await api.agentTools.list(1, 100, filter)
      setAgentTools(page.items)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("agentTools.loadFailed"))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter])

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

      <TableFilterBar
        defs={filterDefs}
        state={filterState}
        onChange={setFilterValue}
        onReset={resetFilters}
        hasActiveFilters={hasActiveFilters}
        clearLabel={t("common.clearFilters")}
        allLabel={t("common.allFilter")}
      />

      <div className="glass-panel">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("agentTools.columnName")}</TableHead>
              <TableHead>{t("agentTools.columnDescription")}</TableHead>
              <TableHead>{t("agentTools.columnStatus")}</TableHead>
              <TableHead>{t("agentTools.columnVisibility")}</TableHead>
              <TableHead>{t("common.actions")}</TableHead>
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
                  {hasActiveFilters ? t("common.noSearchResults") : t("agentTools.empty")}
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
                <TableCell>
                  {hasLevelOnResource("agent_tool", tool.id, "manage") && (
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
