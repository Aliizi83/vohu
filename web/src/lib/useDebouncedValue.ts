import { useEffect, useState } from "react"

// Debounces a value's changes rather than the search input's onChange
// handler — the input itself stays perfectly responsive (React state
// updates immediately on every keystroke), only the value list pages
// react to (and therefore the network request it triggers) lags behind.
export function useDebouncedValue<T>(value: T, delayMs = 300): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs)
    return () => clearTimeout(timer)
  }, [value, delayMs])

  return debounced
}
