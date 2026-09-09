import { useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { useLanguage } from "@/lib/i18n"
import {
  api,
  ApiError,
  streamMessage,
  type ConversationDto,
  type CustomModelDto,
  type MessageDto,
  type SSHConnectionDto,
} from "@/lib/api"
import { cn } from "cn"

// How many messages a page of chat history holds — small enough that
// older history doesn't get fetched until the user actually scrolls up
// for it, large enough that a normal-length conversation loads in one page.
const MESSAGES_PAGE_SIZE = 30

const MODEL_OPTIONS = [
  { label: "Gemini Flash", provider: "gemini", model: "gemini-3.7-flash" },
  { label: "Gemini Flash Lite 3.5", provider: "gemini", model: "gemini-3.5-flash-lite" },
  { label: "Claude Opus 5", provider: "anthropic", model: "claude-opus-5" },
  { label: "Claude Sonnet 5", provider: "anthropic", model: "claude-sonnet-5" },
  { label: "Claude Haiku 4.5", provider: "anthropic", model: "claude-haiku-4-5-20251001" },
  { label: "Custom (OpenAI-compatible)", provider: "openai", model: "" },
] as const

export default function ChatPage() {
  const { t } = useLanguage()
  const [conversations, setConversations] = useState<ConversationDto[] | null>(null)
  const [connections, setConnections] = useState<SSHConnectionDto[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [messages, setMessages] = useState<MessageDto[] | null>(null)
  const [hasMoreMessages, setHasMoreMessages] = useState(false)
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false)

  const [pendingUserContent, setPendingUserContent] = useState<string | null>(null)
  const [streamingText, setStreamingText] = useState("")
  const [isStreaming, setIsStreaming] = useState(false)
  const [input, setInput] = useState("")

  const scrollRef = useRef<HTMLDivElement>(null)
  // The next page to fetch when the user scrolls up for older history —
  // page 1 (the most recent messages) is always loaded up front when a
  // conversation is selected, so this starts at 2.
  const nextOlderPageRef = useRef(2)
  // Set right before a state update that should jump the view to the
  // bottom (a freshly selected conversation, a newly sent/received
  // message) — read and cleared by the layout effect below, since
  // "messages changed" alone doesn't say whether that's from the bottom
  // (scroll down) or the top (scroll-up load, keep position).
  const shouldScrollToBottomRef = useRef(false)
  // Set right before prepending an older page, holding the scroll
  // metrics from just before the DOM grows — the layout effect uses it to
  // keep the same message in view instead of the prepend visually
  // yanking the reader back to a different spot.
  const preserveScrollRef = useRef<{ height: number; top: number } | null>(null)

  const loadConversations = useCallback(async () => {
    try {
      const page = await api.conversations.list(1, 50)
      setConversations(page.items)
      if (page.items.length > 0 && selectedId === null) {
        setSelectedId(page.items[0].id)
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("chat.loadConversationsFailed"))
    }
    // selectedId and t intentionally excluded — this only picks a default
    // once, and t's identity changing on language switch shouldn't
    // re-trigger a network call.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    loadConversations()
    api.sshConnections
      .list(1, 100)
      .then((page) => setConnections(page.items))
      .catch(() => {
        // Non-fatal — the connections list is just a helper hint in the
        // sidebar; chat still works without it.
      })
  }, [loadConversations])

  useEffect(() => {
    if (selectedId === null) {
      setMessages(null)
      setHasMoreMessages(false)
      return
    }
    setMessages(null)
    setHasMoreMessages(false)
    nextOlderPageRef.current = 2
    api.conversations
      .messages(selectedId, 1, MESSAGES_PAGE_SIZE)
      .then((page) => {
        shouldScrollToBottomRef.current = true
        setMessages(page.items)
        setHasMoreMessages(page.hasNextPage)
      })
      .catch((err) => {
        toast.error(err instanceof ApiError ? err.message : t("chat.loadMessagesFailed"))
        setMessages([])
      })
    // t intentionally excluded, same reasoning as loadConversations above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedId])

  // Fires when the reader scrolls near the top of the currently loaded
  // history — fetches the next (older) page and prepends it, preserving
  // scroll position via preserveScrollRef instead of jumping the view.
  const loadOlderMessages = useCallback(() => {
    const el = scrollRef.current
    if (!el || selectedId === null || !hasMoreMessages || loadingOlderMessages) return

    setLoadingOlderMessages(true)
    preserveScrollRef.current = { height: el.scrollHeight, top: el.scrollTop }
    api.conversations
      .messages(selectedId, nextOlderPageRef.current, MESSAGES_PAGE_SIZE)
      .then((page) => {
        nextOlderPageRef.current += 1
        setMessages((prev) => [...page.items, ...(prev ?? [])])
        setHasMoreMessages(page.hasNextPage)
      })
      .catch((err) => {
        preserveScrollRef.current = null
        toast.error(err instanceof ApiError ? err.message : t("chat.loadMessagesFailed"))
      })
      .finally(() => setLoadingOlderMessages(false))
  }, [selectedId, hasMoreMessages, loadingOlderMessages, t])

  function handleScroll() {
    const el = scrollRef.current
    if (!el || el.scrollTop > 80) return
    loadOlderMessages()
  }

  // Scrolls to the bottom while the assistant's reply streams in, or
  // while the optimistic pending user bubble is showing — both only ever
  // add content at the bottom, unlike `messages` changing below.
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [streamingText, pendingUserContent])

  // `messages` changes for two different reasons that need opposite
  // scroll behavior: a freshly loaded conversation or a newly appended
  // turn should jump to the bottom (shouldScrollToBottomRef); an older
  // page prepended from a scroll-up load should instead keep the
  // reader's current position in view (preserveScrollRef).
  useLayoutEffect(() => {
    const el = scrollRef.current
    if (!el) return

    if (preserveScrollRef.current) {
      const { height, top } = preserveScrollRef.current
      el.scrollTop = el.scrollHeight - height + top
      preserveScrollRef.current = null
      return
    }

    if (shouldScrollToBottomRef.current) {
      el.scrollTop = el.scrollHeight
      shouldScrollToBottomRef.current = false
    }
  }, [messages])

  async function handleSend(e: FormEvent) {
    e.preventDefault()
    if (!selectedId || !input.trim() || isStreaming) return

    const content = input.trim()
    setInput("")
    setPendingUserContent(content)
    setStreamingText("")
    setIsStreaming(true)

    await streamMessage(selectedId, content, {
      onChunk: (chunk) => setStreamingText((prev) => prev + chunk),
      onDone: (newMessages) => {
        shouldScrollToBottomRef.current = true
        setMessages((prev) => [...(prev ?? []), ...newMessages])
        setPendingUserContent(null)
        setStreamingText("")
        setIsStreaming(false)
      },
      onError: (message) => {
        toast.error(message)
        setPendingUserContent(null)
        setStreamingText("")
        setIsStreaming(false)
      },
    })
  }

  const selectedConversation = conversations?.find((c) => c.id === selectedId) ?? null

  return (
    <div className="flex h-[calc(100vh-3rem)] gap-4">
      <aside className="flex w-64 shrink-0 flex-col gap-2 overflow-y-auto rounded-md border p-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-muted-foreground">{t("chat.conversations")}</h2>
          <NewConversationDialog
            onCreated={(conv) => {
              setConversations((prev) => [conv, ...(prev ?? [])])
              setSelectedId(conv.id)
            }}
          />
        </div>

        {conversations === null &&
          Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}

        {conversations?.length === 0 && (
          <p className="px-1 py-4 text-sm text-muted-foreground">{t("chat.noConversations")}</p>
        )}

        {conversations?.map((conv) => (
          <button
            key={conv.id}
            onClick={() => setSelectedId(conv.id)}
            className={cn(
              "flex flex-col items-start rounded-md px-3 py-2 text-start text-sm transition-colors",
              conv.id === selectedId
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
            )}
          >
            <span className="w-full truncate font-medium">{conv.title}</span>
            <span className="text-xs opacity-70">
              {conv.provider} · {conv.model}
            </span>
          </button>
        ))}

        {connections.length > 0 && (
          <div className="mt-4 border-t pt-3">
            <h3 className="mb-2 text-xs font-semibold text-muted-foreground">{t("chat.sshConnectionsHeading")}</h3>
            <ul className="space-y-1 text-xs text-muted-foreground">
              {connections.map((c) => (
                <li key={c.id}>
                  #{c.id} {c.name} ({c.host})
                </li>
              ))}
            </ul>
          </div>
        )}
      </aside>

      <section className="flex flex-1 flex-col rounded-md border">
        {!selectedConversation ? (
          <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
            {conversations === null ? t("chat.loading") : t("chat.selectPrompt")}
          </div>
        ) : (
          <>
            <div className="border-b px-4 py-3">
              <h2 className="font-semibold">{selectedConversation.title}</h2>
              <p className="text-xs text-muted-foreground">
                {selectedConversation.provider} · {selectedConversation.model}
              </p>
            </div>

            <div ref={scrollRef} onScroll={handleScroll} className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
              {messages === null &&
                Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-12 w-2/3" />)}

              {loadingOlderMessages && (
                <p className="text-center text-xs text-muted-foreground">{t("chat.loadingOlderMessages")}</p>
              )}

              {messages?.map((msg, i) => (
                <MessageBubble key={i} message={msg} />
              ))}

              {pendingUserContent && (
                <MessageBubble message={{ role: "user", content: pendingUserContent }} />
              )}

              {isStreaming && (
                <MessageBubble
                  message={{ role: "assistant", content: streamingText || "…" }}
                  pending={streamingText === ""}
                />
              )}
            </div>

            <form onSubmit={handleSend} className="flex gap-2 border-t p-3">
              <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={t("chat.composerPlaceholder")}
                disabled={isStreaming}
              />
              <Button type="submit" disabled={isStreaming || !input.trim()}>
                {isStreaming ? t("chat.sending") : t("chat.send")}
              </Button>
            </form>
          </>
        )}
      </section>
    </div>
  )
}

function MessageBubble({ message, pending }: { message: MessageDto; pending?: boolean }) {
  const { t } = useLanguage()

  if (message.role === "tool") {
    return (
      <div className="space-y-1 rounded-md border bg-muted/50 px-3 py-2 text-xs">
        {message.toolResults?.map((result, i) => (
          <div key={i}>
            <span className="font-medium">{result.name}</span>{" "}
            {result.error ? (
              <span className="text-destructive">{t("chat.toolError", { error: result.error })}</span>
            ) : (
              <pre className="mt-1 whitespace-pre-wrap break-words text-muted-foreground">
                {typeof result.result === "string" ? result.result : JSON.stringify(result.result, null, 2)}
              </pre>
            )}
          </div>
        ))}
      </div>
    )
  }

  const isUser = message.role === "user"

  return (
    <div className={cn("flex flex-col gap-1", isUser ? "items-end" : "items-start")}>
      <div
        className={cn(
          "max-w-[75%] rounded-lg px-3 py-2 text-sm whitespace-pre-wrap break-words",
          isUser ? "bg-primary text-primary-foreground" : "bg-muted",
          pending && "text-muted-foreground italic",
        )}
      >
        {message.content}
      </div>
      {message.toolCalls?.map((call) => (
        <span key={call.id} className="text-xs text-muted-foreground">
          {t("chat.callingTool", { name: call.name, args: JSON.stringify(call.arguments) })}
        </span>
      ))}
    </div>
  )
}

// A selection in the model picker is either one of the hardcoded
// MODEL_OPTIONS (key "builtin:<index>") or one of the caller's saved
// custommodel presets (key "custom:<id>") — a plain index into
// MODEL_OPTIONS alone can't represent the dynamic, per-user preset list,
// so every option in the picker gets one of these string keys instead.
function builtinKey(index: number) {
  return `builtin:${index}`
}
function customKey(id: number) {
  return `custom:${id}`
}

function NewConversationDialog({ onCreated }: { onCreated: (conv: ConversationDto) => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState("")
  const [selectedKey, setSelectedKey] = useState(builtinKey(0))
  const [customModel, setCustomModel] = useState("")
  const [loading, setLoading] = useState(false)
  const [presets, setPresets] = useState<CustomModelDto[]>([])

  useEffect(() => {
    if (!open) return
    api.customModels
      .listAvailable()
      .then(setPresets)
      .catch(() => {
        // Non-fatal — the hardcoded MODEL_OPTIONS still work without it.
      })
  }, [open])

  const selectedPreset = selectedKey.startsWith("custom:")
    ? presets.find((p) => customKey(p.id) === selectedKey)
    : undefined
  const builtinIndex = selectedKey.startsWith("builtin:") ? Number(selectedKey.slice("builtin:".length)) : -1
  const builtinOption = builtinIndex >= 0 ? MODEL_OPTIONS[builtinIndex] : undefined
  const isFreeformCustom = builtinOption?.provider === "openai" && builtinOption.model === ""

  const selectedLabel = selectedPreset?.name ?? builtinOption?.label ?? t("chat.modelLabel")

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()

    const provider = selectedPreset ? "openai" : builtinOption?.provider
    const model = selectedPreset ? selectedPreset.modelName : isFreeformCustom ? customModel.trim() : builtinOption?.model
    if (!provider || !model) return

    setLoading(true)
    try {
      const conv = await api.conversations.create({
        title: title.trim() || t("chat.titlePlaceholder"),
        provider,
        model,
        customModelId: selectedPreset?.id,
      })
      toast.success(t("chat.conversationCreated"))
      setOpen(false)
      setTitle("")
      setCustomModel("")
      onCreated(conv)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("chat.createConversationFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm">{t("chat.newChat")}</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("chat.newConversationDialogTitle")}</DialogTitle>
            <DialogDescription>{t("chat.newConversationDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="conv-title">{t("chat.titleLabel")}</Label>
              <Input
                id="conv-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder={t("chat.titlePlaceholder")}
              />
            </div>
            <div className="space-y-2">
              <Label>{t("chat.modelLabel")}</Label>
              <Select value={selectedKey} onValueChange={(value) => setSelectedKey(value ?? builtinKey(0))}>
                <SelectTrigger className="w-full">
                  <SelectValue>{() => selectedLabel}</SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {MODEL_OPTIONS.map((opt, i) => (
                    <SelectItem key={builtinKey(i)} value={builtinKey(i)}>
                      {opt.label}
                    </SelectItem>
                  ))}
                  {presets.map((preset) => (
                    <SelectItem key={customKey(preset.id)} value={customKey(preset.id)}>
                      {preset.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {isFreeformCustom && (
              <div className="space-y-2">
                <Label htmlFor="conv-custom-model">{t("chat.modelNameLabel")}</Label>
                <Input
                  id="conv-custom-model"
                  value={customModel}
                  onChange={(e) => setCustomModel(e.target.value)}
                  placeholder={t("chat.modelNamePlaceholder")}
                  required
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || (isFreeformCustom && !customModel.trim())}>
              {loading ? t("common.creating") : t("common.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
