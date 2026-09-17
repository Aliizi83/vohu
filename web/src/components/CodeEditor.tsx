import * as monaco from "monaco-editor"
import editorWorker from "monaco-editor/editor/editor.worker?worker"
import Editor, { loader } from "@monaco-editor/react"
import { useTheme } from "next-themes"
import { useEffect, useRef, useState } from "react"
import { api } from "@/lib/api"
import { useLanguage } from "@/lib/i18n"
import { cn } from "cn"

// Self-hosted, not the @monaco-editor/react default of fetching from a
// CDN at runtime — same reasoning as Markdown.tsx importing Prism's
// grammars directly instead of pulling them in over the network.
self.MonacoEnvironment = {
  getWorker: () => new editorWorker(),
}
loader.config({ monaco })

// How long to wait after the last keystroke before asking the backend to
// compile the source — every check is a real `go build` (see
// customtool.Handler.CheckSource), so this keeps it from firing on every
// character.
const CHECK_DEBOUNCE_MS = 800

export function CodeEditor({
  value,
  onChange,
  language = "go",
  height = "300px",
}: {
  value: string
  onChange: (value: string) => void
  language?: string
  height?: string
}) {
  const { resolvedTheme } = useTheme()
  const { t } = useLanguage()
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null)
  const latestValueRef = useRef(value)
  useEffect(() => {
    latestValueRef.current = value
  }, [value])

  const [checking, setChecking] = useState(false)
  const [errorCount, setErrorCount] = useState<number | null>(null)

  useEffect(() => {
    if (language !== "go") return

    if (!value.trim()) {
      setErrorCount(null)
      const model = editorRef.current?.getModel()
      if (model) monaco.editor.setModelMarkers(model, "go-build", [])
      return
    }

    const timer = setTimeout(async () => {
      setChecking(true)
      try {
        const result = await api.customTools.checkSource(value)
        // A newer keystroke started another check while this one was in
        // flight — its result will land instead, so this one is stale.
        if (latestValueRef.current !== value) return

        setErrorCount(result.diagnostics.length)
        const model = editorRef.current?.getModel()
        if (model) {
          monaco.editor.setModelMarkers(
            model,
            "go-build",
            result.diagnostics.map((d) => ({
              startLineNumber: d.line,
              startColumn: d.column,
              endLineNumber: d.line,
              endColumn: d.column + 1,
              message: d.message,
              severity: monaco.MarkerSeverity.Error,
            })),
          )
        }
      } catch {
        // Best-effort — a failed check request shouldn't block editing.
      } finally {
        setChecking(false)
      }
    }, CHECK_DEBOUNCE_MS)

    return () => clearTimeout(timer)
  }, [value, language])

  return (
    <div className="space-y-1">
      <div className="overflow-hidden rounded-lg border border-input">
        <Editor
          height={height}
          language={language}
          value={value}
          onChange={(v) => onChange(v ?? "")}
          onMount={(editor) => {
            editorRef.current = editor
          }}
          theme={resolvedTheme === "dark" ? "vs-dark" : "light"}
          options={{
            minimap: { enabled: false },
            fontSize: 13,
            scrollBeyondLastLine: false,
            automaticLayout: true,
          }}
        />
      </div>
      {language === "go" && (
        <p className={cn("text-xs", errorCount ? "text-destructive" : "text-muted-foreground")}>
          {checking
            ? t("customTools.checkingSource")
            : errorCount === null
              ? " "
              : errorCount === 0
                ? t("customTools.sourceOk")
                : t("customTools.sourceErrorCount", { count: String(errorCount) })}
        </p>
      )}
    </div>
  )
}
