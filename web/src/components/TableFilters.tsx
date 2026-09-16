import { useMemo, useState } from "react"
import { ChevronDownIcon, XIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SearchInput } from "@/components/SearchInput"
import { useDebouncedValue } from "@/lib/useDebouncedValue"
import type { DynamicFilter } from "@/lib/api"

// A "no filter" sentinel for single-select fields — base-ui's Select
// doesn't treat an empty string specially, so a real (non-empty) value is
// needed to represent "every option" and get mapped back to "omit this
// filter" when building the DynamicFilter.
const SELECT_ALL = "__all__"

export type FilterOption = { value: string; label: string }

export type FilterFieldDef =
  | { key: string; label: string; kind: "search" }
  | { key: string; label: string; kind: "select"; options: FilterOption[] }
  | { key: string; label: string; kind: "multiSelect"; options: FilterOption[] }

type FilterValue = string | string[]
type FilterState = Record<string, FilterValue>

function isActive(value: FilterValue | undefined): boolean {
  if (value === undefined) return false
  return Array.isArray(value) ? value.length > 0 : value !== ""
}

function buildDynamicFilter(defs: FilterFieldDef[], state: FilterState): DynamicFilter | undefined {
  const filters: NonNullable<DynamicFilter["filters"]> = {}
  for (const def of defs) {
    const value = state[def.key]
    if (!isActive(value)) continue
    if (def.kind === "search") {
      filters[def.key] = { type: "contains", from: value as string }
    } else if (def.kind === "select") {
      filters[def.key] = { type: "equals", from: value as string }
    } else {
      filters[def.key] = { type: "in", from: (value as string[]).join(",") }
    }
  }
  return Object.keys(filters).length > 0 ? { filters } : undefined
}

// useTableFilters owns the raw per-field UI state and derives the
// DynamicFilter the backend understands. The whole state object is
// debounced as one unit (not just the search fields) — a select/multiSelect
// change riding along on the same short debounce as a keystroke costs
// nothing and keeps this simple, see useDebouncedValue.
export function useTableFilters(defs: FilterFieldDef[]) {
  const [state, setState] = useState<FilterState>({})
  const debouncedState = useDebouncedValue(state)
  const filter = useMemo(() => buildDynamicFilter(defs, debouncedState), [defs, debouncedState])
  // Driven by the same (debounced) state the filter itself is built from,
  // so "there are active filters" never disagrees with "the query is
  // currently filtered" — no flash of the wrong empty-state copy while a
  // keystroke is still debouncing.
  const hasActiveFilters = filter !== undefined

  function setValue(key: string, value: FilterValue) {
    setState((prev) => ({ ...prev, [key]: value }))
  }

  function reset() {
    setState({})
  }

  return { state, setValue, filter, hasActiveFilters, reset }
}

export function TableFilterBar({
  defs,
  state,
  onChange,
  onReset,
  hasActiveFilters,
  clearLabel,
  allLabel,
}: {
  defs: FilterFieldDef[]
  state: FilterState
  onChange: (key: string, value: FilterValue) => void
  onReset: () => void
  hasActiveFilters: boolean
  clearLabel: string
  allLabel: string
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      {defs.map((def) => {
        if (def.kind === "search") {
          return (
            <SearchInput
              key={def.key}
              value={(state[def.key] as string) ?? ""}
              onChange={(value) => onChange(def.key, value)}
              placeholder={def.label}
              className="w-48"
            />
          )
        }

        if (def.kind === "select") {
          const value = (state[def.key] as string) || SELECT_ALL
          return (
            <Select
              key={def.key}
              value={value}
              onValueChange={(next) => onChange(def.key, next === SELECT_ALL ? "" : (next ?? ""))}
            >
              <SelectTrigger className="w-40">
                <SelectValue placeholder={def.label}>
                  {(v: string) => (v === SELECT_ALL ? def.label : (def.options.find((o) => o.value === v)?.label ?? def.label))}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={SELECT_ALL}>{allLabel}</SelectItem>
                {def.options.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )
        }

        const selected = (state[def.key] as string[] | undefined) ?? []
        return (
          <MultiSelectFilter
            key={def.key}
            label={def.label}
            options={def.options}
            selected={selected}
            onChange={(next) => onChange(def.key, next)}
          />
        )
      })}

      {hasActiveFilters && (
        <Button variant="ghost" size="sm" onClick={onReset}>
          <XIcon />
          {clearLabel}
        </Button>
      )}
    </div>
  )
}

function MultiSelectFilter({
  label,
  options,
  selected,
  onChange,
}: {
  label: string
  options: FilterOption[]
  selected: string[]
  onChange: (values: string[]) => void
}) {
  const summary = selected.length === 0 ? label : `${label} (${selected.length})`

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button variant="outline" size="sm">
            {summary}
            <ChevronDownIcon />
          </Button>
        }
      />
      <DropdownMenuContent>
        {options.map((option) => (
          <DropdownMenuCheckboxItem
            key={option.value}
            checked={selected.includes(option.value)}
            onCheckedChange={(checked) =>
              onChange(checked ? [...selected, option.value] : selected.filter((v) => v !== option.value))
            }
          >
            {option.label}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
