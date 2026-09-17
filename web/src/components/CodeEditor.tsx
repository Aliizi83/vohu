import * as monaco from "monaco-editor"
import editorWorker from "monaco-editor/editor/editor.worker?worker"
import Editor, { loader } from "@monaco-editor/react"
import { useTheme } from "next-themes"

// Self-hosted, not the @monaco-editor/react default of fetching from a
// CDN at runtime — same reasoning as Markdown.tsx importing Prism's
// grammars directly instead of pulling them in over the network.
self.MonacoEnvironment = {
  getWorker: () => new editorWorker(),
}
loader.config({ monaco })

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

  return (
    <div className="overflow-hidden rounded-lg border border-input">
      <Editor
        height={height}
        language={language}
        value={value}
        onChange={(v) => onChange(v ?? "")}
        theme={resolvedTheme === "dark" ? "vs-dark" : "light"}
        options={{
          minimap: { enabled: false },
          fontSize: 13,
          scrollBeyondLastLine: false,
          automaticLayout: true,
        }}
      />
    </div>
  )
}
