import { useEffect, useRef, useState } from "react"
import { FitAddon } from "@xterm/addon-fit"
import { Terminal } from "@xterm/xterm"
import "@xterm/xterm/css/xterm.css"
import { ArrowLeftIcon } from "lucide-react"
import { useTheme } from "next-themes"
import { Link, useParams } from "react-router-dom"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { API_BASE, api, ApiError } from "@/lib/api"
import { useLanguage } from "@/lib/i18n"

const DARK_THEME = { background: "#1e1e1e", foreground: "#d4d4d4" }
const LIGHT_THEME = { background: "#ffffff", foreground: "#1e1e1e" }

type Status = "connecting" | "connected" | "disconnected" | "error"

// A full, unrestricted interactive shell — not run through
// internal/tools/command.Policy's allow-list the way ssh_execute is (a
// raw PTY can't be filtered command-by-command). Access is gated at
// "write" level on the connection itself, same bar as ssh_execute; see
// terminal.Handler's doc comments for the rest of the trust model.
export default function TerminalPage() {
  const { id } = useParams<{ id: string }>()
  const { t } = useLanguage()
  const { resolvedTheme } = useTheme()
  const containerRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)
  const [status, setStatus] = useState<Status>("connecting")

  useEffect(() => {
    if (termRef.current) {
      termRef.current.options.theme = resolvedTheme === "dark" ? DARK_THEME : LIGHT_THEME
    }
  }, [resolvedTheme])

  useEffect(() => {
    if (!id || !containerRef.current) return
    let disposed = false

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 13,
      fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace",
      theme: resolvedTheme === "dark" ? DARK_THEME : LIGHT_THEME,
    })
    termRef.current = term
    const fitAddon = new FitAddon()
    term.loadAddon(fitAddon)
    term.open(containerRef.current)
    fitAddon.fit()

    let ws: WebSocket | null = null

    async function connect() {
      try {
        const { ticket } = await api.sshConnections.terminalTicket(Number(id))
        if (disposed) return

        const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:"
        ws = new WebSocket(
          `${wsProtocol}//${window.location.host}${API_BASE}/ssh-connections/${id}/terminal-ws?ticket=${ticket}`,
        )

        ws.onopen = () => {
          setStatus("connected")
          ws?.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }))
        }
        ws.onmessage = (event) => term.write(event.data)
        ws.onclose = () => setStatus((prev) => (prev === "connecting" ? "error" : "disconnected"))
        ws.onerror = () => setStatus("error")

        term.onData((data) => ws?.send(JSON.stringify({ type: "input", data })))
      } catch (err) {
        setStatus("error")
        toast.error(err instanceof ApiError ? err.message : t("terminal.connectFailed"))
      }
    }
    connect()

    function handleResize() {
      fitAddon.fit()
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows }))
      }
    }
    window.addEventListener("resize", handleResize)

    return () => {
      disposed = true
      window.removeEventListener("resize", handleResize)
      ws?.close()
      term.dispose()
      termRef.current = null
    }
    // resolvedTheme intentionally excluded — the theme-sync effect above
    // handles it without tearing down and reconnecting the session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  const theme = resolvedTheme === "dark" ? DARK_THEME : LIGHT_THEME

  return (
    <div dir="ltr" className="flex h-screen flex-col" style={{ background: theme.background, color: theme.foreground }}>
      <div className="flex items-center gap-3 border-b border-current/10 px-4 py-2">
        <Button variant="ghost" size="sm" nativeButton={false} render={<Link to="/ssh-connections" />}>
          <ArrowLeftIcon />
          {t("terminal.back")}
        </Button>
        <span className="text-sm opacity-60">
          {status === "connecting" && t("terminal.connecting")}
          {status === "connected" && t("terminal.connected")}
          {status === "disconnected" && t("terminal.disconnected")}
          {status === "error" && t("terminal.connectFailed")}
        </span>
      </div>
      <div ref={containerRef} className="min-h-0 flex-1 p-2" />
    </div>
  )
}
