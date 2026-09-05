import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { en } from "@/locales/en"
import { fa } from "@/locales/fa"

export type Language = "en" | "fa"

const dictionaries: Record<Language, typeof en> = { en, fa }
const RTL_LANGUAGES: readonly Language[] = ["fa"]
const STORAGE_KEY = "vohu.language"

export const LANGUAGE_OPTIONS: { value: Language; label: string }[] = [
  { value: "en", label: "English" },
  { value: "fa", label: "فارسی" },
]

interface LanguageContextValue {
  language: Language
  setLanguage: (lang: Language) => void
  dir: "ltr" | "rtl"
  t: (path: string, vars?: Record<string, string | number>) => string
}

const LanguageContext = createContext<LanguageContextValue | null>(null)

function resolvePath(dict: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc && typeof acc === "object" && key in acc) {
      return (acc as Record<string, unknown>)[key]
    }
    return undefined
  }, dict)
}

function interpolate(template: string, vars?: Record<string, string | number>): string {
  if (!vars) return template
  return template.replace(/\{\{(\w+)\}\}/g, (_, key: string) => String(vars[key] ?? ""))
}

function isLanguage(value: string | null): value is Language {
  return value === "en" || value === "fa"
}

function detectInitialLanguage(): Language {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (isLanguage(stored)) return stored
  return navigator.language.toLowerCase().startsWith("fa") ? "fa" : "en"
}

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>(detectInitialLanguage)

  const dir: "ltr" | "rtl" = RTL_LANGUAGES.includes(language) ? "rtl" : "ltr"

  useEffect(() => {
    document.documentElement.lang = language
    document.documentElement.dir = dir
  }, [language, dir])

  function setLanguage(lang: Language) {
    setLanguageState(lang)
    localStorage.setItem(STORAGE_KEY, lang)
  }

  const t = useMemo(() => {
    const dict = dictionaries[language]
    return (path: string, vars?: Record<string, string | number>) => {
      const value = resolvePath(dict, path)
      if (typeof value !== "string") return path
      return interpolate(value, vars)
    }
  }, [language])

  return (
    <LanguageContext.Provider value={{ language, setLanguage, dir, t }}>{children}</LanguageContext.Provider>
  )
}

export function useLanguage() {
  const ctx = useContext(LanguageContext)
  if (!ctx) throw new Error("useLanguage must be used within a LanguageProvider")
  return ctx
}
