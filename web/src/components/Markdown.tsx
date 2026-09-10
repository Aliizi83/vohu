import { useState, type ComponentProps } from "react"
import { CheckIcon, CopyIcon } from "lucide-react"
import { useTheme } from "next-themes"
import ReactMarkdown from "react-markdown"
import { PrismAsyncLight as SyntaxHighlighter } from "react-syntax-highlighter"
import oneDark from "react-syntax-highlighter/dist/esm/styles/prism/one-dark"
import oneLight from "react-syntax-highlighter/dist/esm/styles/prism/one-light"
import bash from "react-syntax-highlighter/dist/esm/languages/prism/bash"
import c from "react-syntax-highlighter/dist/esm/languages/prism/c"
import cpp from "react-syntax-highlighter/dist/esm/languages/prism/cpp"
import csharp from "react-syntax-highlighter/dist/esm/languages/prism/csharp"
import css from "react-syntax-highlighter/dist/esm/languages/prism/css"
import diff from "react-syntax-highlighter/dist/esm/languages/prism/diff"
import docker from "react-syntax-highlighter/dist/esm/languages/prism/docker"
import go from "react-syntax-highlighter/dist/esm/languages/prism/go"
import graphql from "react-syntax-highlighter/dist/esm/languages/prism/graphql"
import ini from "react-syntax-highlighter/dist/esm/languages/prism/ini"
import java from "react-syntax-highlighter/dist/esm/languages/prism/java"
import javascript from "react-syntax-highlighter/dist/esm/languages/prism/javascript"
import json from "react-syntax-highlighter/dist/esm/languages/prism/json"
import jsx from "react-syntax-highlighter/dist/esm/languages/prism/jsx"
import kotlin from "react-syntax-highlighter/dist/esm/languages/prism/kotlin"
import lua from "react-syntax-highlighter/dist/esm/languages/prism/lua"
import markdown from "react-syntax-highlighter/dist/esm/languages/prism/markdown"
import markup from "react-syntax-highlighter/dist/esm/languages/prism/markup"
import php from "react-syntax-highlighter/dist/esm/languages/prism/php"
import python from "react-syntax-highlighter/dist/esm/languages/prism/python"
import ruby from "react-syntax-highlighter/dist/esm/languages/prism/ruby"
import rust from "react-syntax-highlighter/dist/esm/languages/prism/rust"
import sql from "react-syntax-highlighter/dist/esm/languages/prism/sql"
import swift from "react-syntax-highlighter/dist/esm/languages/prism/swift"
import tsx from "react-syntax-highlighter/dist/esm/languages/prism/tsx"
import typescript from "react-syntax-highlighter/dist/esm/languages/prism/typescript"
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml"
import remarkGfm from "remark-gfm"
import { cn } from "cn"

// Registered once, module-level — the most commonly used languages, not
// Prism's full ~290-grammar set, so the async-light build only ever loads
// what's actually likely to show up in a chat message. aliases beyond the
// registered name (e.g. "js" -> "javascript") are handled in
// resolveLanguage below, since model output and user-typed fences use
// either interchangeably.
const LANGUAGES: Record<string, unknown> = {
  bash,
  c,
  cpp,
  csharp,
  css,
  diff,
  docker,
  go,
  graphql,
  ini,
  java,
  javascript,
  json,
  jsx,
  kotlin,
  lua,
  markdown,
  markup,
  php,
  python,
  ruby,
  rust,
  sql,
  swift,
  tsx,
  typescript,
  yaml,
}
for (const [name, grammar] of Object.entries(LANGUAGES)) {
  SyntaxHighlighter.registerLanguage(name, grammar as Parameters<typeof SyntaxHighlighter.registerLanguage>[1])
}

const LANGUAGE_ALIASES: Record<string, string> = {
  js: "javascript",
  mjs: "javascript",
  cjs: "javascript",
  ts: "typescript",
  py: "python",
  rb: "ruby",
  sh: "bash",
  shell: "bash",
  zsh: "bash",
  yml: "yaml",
  golang: "go",
  html: "markup",
  xml: "markup",
  svg: "markup",
  dockerfile: "docker",
  "c++": "cpp",
  "c#": "csharp",
  md: "markdown",
  cs: "csharp",
}

function resolveLanguage(lang: string | undefined): string | undefined {
  if (!lang) return undefined
  const normalized = lang.toLowerCase()
  const resolved = LANGUAGE_ALIASES[normalized] ?? normalized
  return resolved in LANGUAGES ? resolved : undefined
}

function CodeBlock({ language, code }: { language: string | undefined; code: string }) {
  const { resolvedTheme } = useTheme()
  const [copied, setCopied] = useState(false)
  const resolved = resolveLanguage(language)

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    } catch {
      // Clipboard access can be denied by the browser (permissions,
      // insecure context, ...) — nothing useful to recover into beyond
      // leaving the button in its un-copied state.
    }
  }

  return (
    <div dir="ltr" className="group relative my-1.5 overflow-hidden rounded-md border text-start">
      <div className="flex items-center justify-between bg-muted px-3 py-1 text-xs text-muted-foreground">
        <span>{language || "text"}</span>
        <button
          type="button"
          onClick={handleCopy}
          className="flex items-center gap-1 rounded px-1.5 py-0.5 opacity-0 transition-opacity hover:bg-accent hover:text-accent-foreground group-hover:opacity-100"
        >
          {copied ? <CheckIcon className="size-3" /> : <CopyIcon className="size-3" />}
        </button>
      </div>
      {resolved ? (
        <SyntaxHighlighter
          language={resolved}
          style={resolvedTheme === "dark" ? oneDark : oneLight}
          customStyle={{ margin: 0, borderRadius: 0, fontSize: "0.8rem", overflowX: "auto" }}
        >
          {code}
        </SyntaxHighlighter>
      ) : (
        <pre className="overflow-x-auto bg-card px-3 py-2 font-mono text-xs">{code}</pre>
      )}
    </div>
  )
}

// Message renders a message's markdown content — used for both the
// user's own sent messages and the assistant's, so a fenced code block
// either side writes gets the same syntax-highlighted treatment. dir is
// forced to "ltr" inside CodeBlock regardless of the page's current
// language: code syntax is always left-to-right, independent of Farsi/
// English UI.
export function Markdown({ content, className }: { content: string; className?: string }) {
  return (
    <div className={cn("markdown-content space-y-2 text-sm leading-relaxed", className)}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: (props: ComponentProps<"p">) => <p className="whitespace-pre-wrap break-words" {...props} />,
          a: (props: ComponentProps<"a">) => (
            <a className="underline underline-offset-2" target="_blank" rel="noreferrer" {...props} />
          ),
          ul: (props: ComponentProps<"ul">) => <ul className="ms-5 list-disc space-y-0.5" {...props} />,
          ol: (props: ComponentProps<"ol">) => <ol className="ms-5 list-decimal space-y-0.5" {...props} />,
          table: (props: ComponentProps<"table">) => (
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-xs" {...props} />
            </div>
          ),
          th: (props: ComponentProps<"th">) => (
            <th className="border px-2 py-1 text-start font-medium" {...props} />
          ),
          td: (props: ComponentProps<"td">) => <td className="border px-2 py-1" {...props} />,
          code(props) {
            const { children, className: codeClassName } = props
            const match = /language-(\w+)/.exec(codeClassName || "")
            const text = String(children).replace(/\n$/, "")

            // react-markdown v9+ dropped the `inline` prop from code's
            // signature — a fenced block is distinguished from inline
            // code by having a "language-xxx" class (from the ```lang
            // fence) or, lacking a language, by actually containing a
            // newline; single-backtick inline code never does.
            if (match || text.includes("\n")) {
              return <CodeBlock language={match?.[1]} code={text} />
            }
            return (
              <code className="rounded bg-foreground/10 px-1 py-0.5 font-mono text-xs" dir="ltr">
                {children}
              </code>
            )
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
