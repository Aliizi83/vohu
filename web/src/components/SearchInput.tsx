import { Search } from "lucide-react"
import { Input } from "@/components/ui/input"

// A plain text Input with a leading search icon — used by every list page
// to drive a "contains" filter through the backend's dynamic query
// system (see api.ts's DynamicFilter / shared.ApplyDynamicFilter on the
// Go side). ps-8 (padding-inline-start) rather than pl-8 so the icon and
// its space stay on the correct edge under RTL too.
export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  className?: string
}) {
  return (
    <div className={`relative ${className ?? ""}`}>
      <Search className="pointer-events-none absolute inset-y-0 start-2.5 my-auto size-4 text-muted-foreground" />
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="ps-8"
      />
    </div>
  )
}
