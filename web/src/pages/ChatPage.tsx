import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState, type FormEvent } from "react"
import { ArchiveIcon, ArchiveRestoreIcon, ChevronDownIcon, PencilIcon, Trash2Icon } from "lucide-react"
import { toast } from "sonner"
import { Markdown } from "@/components/Markdown"
import { useConfirm } from "@/components/ConfirmDialog"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useLanguage } from "@/lib/i18n"
import {
  api,
  ApiError,
  streamMessage,
  type ConversationDto,
  type CustomModelDto,
  type MessageDto,
  type SSHConnectionDto,
  type ToolCallDto,
  type ToolResultDto,
} from "@/lib/api"
import { cn } from "cn"

// How many messages a page of chat history holds — small enough that
// older history doesn't get fetched until the user actually scrolls up
// for it, large enough that a normal-length conversation loads in one page.
const MESSAGES_PAGE_SIZE = 30

// A tool call that's been requested this turn, shown the instant it's
// requested (result undefined) and updated in place once its result
// arrives — rather than only appearing once the whole turn is done.
interface LiveToolCall {
  call: ToolCallDto
  result?: ToolResultDto
}

// One conversation's in-flight-send state — see the streamStates doc
// comment in ChatPage for why this is keyed per-conversation rather than
// three plain useState values.
interface StreamState {
  pendingContent: string
  streamingText: string
  isStreaming: boolean
  toolCalls: LiveToolCall[]
}

const EMPTY_STREAM_STATE: StreamState = { pendingContent: "", streamingText: "", isStreaming: false, toolCalls: [] }

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
  const { confirm, confirmDialog } = useConfirm()
  const [conversations, setConversations] = useState<ConversationDto[] | null>(null)
  const [showArchived, setShowArchived] = useState(false)
  const [renamingId, setRenamingId] = useState<number | null>(null)
  const [renameValue, setRenameValue] = useState("")
  const [connections, setConnections] = useState<SSHConnectionDto[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [messages, setMessages] = useState<MessageDto[] | null>(null)
  const [hasMoreMessages, setHasMoreMessages] = useState(false)
  const [loadingOlderMessages, setLoadingOlderMessages] = useState(false)

  // streamStates holds one entry per conversation that currently has a
  // send in flight, keyed by conversation ID — not three plain useState
  // values, which is what made only one conversation's send able to be
  // "in progress" at a time app-wide: switching conversations while one
  // streamed disabled the composer everywhere (isStreaming was global)
  // and, worse, would have appended that reply to whichever conversation
  // happened to be selected when it finished (see selectedIdRef and
  // onDone below — messages only ever gets a reply appended when the
  // send that produced it belongs to the *currently selected*
  // conversation; a send finishing in the background is still safely
  // persisted server-side regardless, and shows up the next time that
  // conversation is selected via the normal fetch-on-select below).
  const [streamStates, setStreamStates] = useState<Record<number, StreamState>>({})
  const [input, setInput] = useState("")

  // Always the latest selectedId, readable from inside streamMessage's
  // callbacks — those close over whatever selectedId was at send time,
  // which is stale by the time a reply actually arrives if the user has
  // since switched conversations.
  const selectedIdRef = useRef<number | null>(null)
  useEffect(() => {
    selectedIdRef.current = selectedId
  }, [selectedId])

  const currentStream = (selectedId !== null && streamStates[selectedId]) || EMPTY_STREAM_STATE

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

  // loadConversations re-selects the current selection if it's still in
  // the fetched page, otherwise falls back to the first item — the only
  // case that matters in practice is toggling showArchived, since active
  // and archived conversations are disjoint sets and a selection from one
  // is never present in the other.
  const loadConversations = useCallback(
    async (archived: boolean) => {
      try {
        const page = await api.conversations.list(1, 50, archived)
        setConversations(page.items)
        setSelectedId((prev) => (prev !== null && page.items.some((c) => c.id === prev) ? prev : (page.items[0]?.id ?? null)))
      } catch (err) {
        toast.error(err instanceof ApiError ? err.message : t("chat.loadConversationsFailed"))
      }
    },
    // t intentionally excluded — its identity changing on language switch
    // shouldn't re-trigger a network call.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  useEffect(() => {
    loadConversations(showArchived)
  }, [loadConversations, showArchived])

  useEffect(() => {
    api.sshConnections
      .list(1, 100)
      .then((page) => setConnections(page.items))
      .catch(() => {
        // Non-fatal — the connections list is just a helper hint in the
        // sidebar; chat still works without it.
      })
  }, [])

  async function commitRename(id: number) {
    const title = renameValue.trim()
    setRenamingId(null)
    if (!title) return
    try {
      const updated = await api.conversations.update(id, { title })
      setConversations((prev) => prev?.map((c) => (c.id === id ? updated : c)) ?? prev)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("chat.renameFailed"))
    }
  }

  async function handleArchiveToggle(conv: ConversationDto) {
    try {
      await api.conversations.update(conv.id, { archived: !conv.archived })
      toast.success(conv.archived ? t("chat.unarchived", { title: conv.title }) : t("chat.archived", { title: conv.title }))
      loadConversations(showArchived)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("chat.archiveFailed"))
    }
  }

  async function handleDeleteConversation(conv: ConversationDto) {
    const ok = await confirm({ description: t("chat.confirmDelete", { title: conv.title }) })
    if (!ok) return
    try {
      await api.conversations.remove(conv.id)
      toast.success(t("chat.conversationDeleted"))
      loadConversations(showArchived)
    } catch (err) {
      // DELETE is idempotent — a 404 here just means it's already gone
      // (e.g. a double-submitted click), which is exactly the end state
      // the user asked for, not a failure worth alarming them over.
      if (err instanceof ApiError && err.status === 404) {
        loadConversations(showArchived)
        return
      }
      toast.error(err instanceof ApiError ? err.message : t("chat.deleteConversationFailed"))
    }
  }

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
  // add content at the bottom, unlike `messages` changing below. Reads
  // off currentStream (the *selected* conversation's stream state) so
  // this doesn't fire for a send progressing in some other, unselected
  // conversation.
  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [currentStream.streamingText, currentStream.pendingContent])

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

  async function handleSend(e?: FormEvent) {
    e?.preventDefault()
    if (!selectedId || !input.trim() || currentStream.isStreaming) return

    // Captured once, up front — streamMessage's callbacks fire later,
    // possibly after the user has selected a different conversation, so
    // they can't rely on reading selectedId directly (stale closure).
    const conversationId = selectedId
    const content = input.trim()
    setInput("")
    setStreamStates((prev) => ({
      ...prev,
      [conversationId]: { pendingContent: content, streamingText: "", isStreaming: true, toolCalls: [] },
    }))

    await streamMessage(conversationId, content, {
      onChunk: (chunk) => {
        setStreamStates((prev) => ({
          ...prev,
          [conversationId]: {
            ...(prev[conversationId] ?? EMPTY_STREAM_STATE),
            streamingText: (prev[conversationId]?.streamingText ?? "") + chunk,
          },
        }))
      },
      onToolCall: (call) => {
        setStreamStates((prev) => {
          const current = prev[conversationId] ?? EMPTY_STREAM_STATE
          return {
            ...prev,
            [conversationId]: { ...current, toolCalls: [...current.toolCalls, { call }] },
          }
        })
      },
      onToolResult: (result) => {
        setStreamStates((prev) => {
          const current = prev[conversationId] ?? EMPTY_STREAM_STATE
          return {
            ...prev,
            [conversationId]: {
              ...current,
              toolCalls: current.toolCalls.map((tc) =>
                tc.call.id === result.toolCallId ? { ...tc, result } : tc,
              ),
            },
          }
        })
      },
      onTitleChanged: (title) => {
        setConversations((prev) => prev?.map((c) => (c.id === conversationId ? { ...c, title } : c)) ?? prev)
      },
      onDone: (newMessages) => {
        // Only the currently-selected conversation's `messages` array is
        // what's on screen — appending here when the user has switched
        // away would inject this reply into whatever *other*
        // conversation they're now looking at. The reply is already
        // safely persisted server-side regardless; switching back to
        // this conversation later fetches it normally (see the
        // selectedId effect above).
        if (selectedIdRef.current === conversationId) {
          shouldScrollToBottomRef.current = true
          setMessages((prev) => [...(prev ?? []), ...newMessages])
        }
        setStreamStates((prev) => {
          const { [conversationId]: _done, ...rest } = prev
          return rest
        })
      },
      onError: (message) => {
        toast.error(message)
        setStreamStates((prev) => {
          const { [conversationId]: _failed, ...rest } = prev
          return rest
        })
      },
    })
  }

  const selectedConversation = conversations?.find((c) => c.id === selectedId) ?? null

  return (
    <div className="flex h-[calc(100vh-3rem)] gap-4">
      {confirmDialog}
      <aside className="flex w-64 shrink-0 flex-col gap-2 overflow-y-auto rounded-md border p-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-muted-foreground">{t("chat.conversations")}</h2>
          {!showArchived && (
            <NewConversationDialog
              onCreated={(conv) => {
                setConversations((prev) => [conv, ...(prev ?? [])])
                setSelectedId(conv.id)
              }}
            />
          )}
        </div>

        <Tabs value={showArchived ? "archived" : "active"} onValueChange={(v) => setShowArchived(v === "archived")}>
          <TabsList className="w-full">
            <TabsTrigger value="active" className="flex-1">
              {t("chat.tabActive")}
            </TabsTrigger>
            <TabsTrigger value="archived" className="flex-1">
              {t("chat.tabArchived")}
            </TabsTrigger>
          </TabsList>
        </Tabs>

        {conversations === null &&
          Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-9 w-full" />)}

        {conversations?.length === 0 && (
          <p className="px-1 py-4 text-sm text-muted-foreground">
            {showArchived ? t("chat.noArchivedConversations") : t("chat.noConversations")}
          </p>
        )}

        {conversations?.map((conv) => (
          <ContextMenu key={conv.id}>
            <ContextMenuTrigger className="contents">
              {renamingId === conv.id ? (
                <input
                  autoFocus
                  value={renameValue}
                  onChange={(e) => setRenameValue(e.target.value)}
                  onBlur={() => commitRename(conv.id)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault()
                      commitRename(conv.id)
                    } else if (e.key === "Escape") {
                      setRenamingId(null)
                    }
                  }}
                  className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                />
              ) : (
                <button
                  onClick={() => setSelectedId(conv.id)}
                  className={cn(
                    "flex w-full flex-col items-start rounded-md px-3 py-2 text-start text-sm transition-colors",
                    conv.id === selectedId
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground",
                  )}
                >
                  <span className="flex w-full items-center gap-1.5">
                    <span className="truncate font-medium">{conv.title}</span>
                    {streamStates[conv.id]?.isStreaming && (
                      <span
                        className="size-1.5 shrink-0 animate-pulse rounded-full bg-primary"
                        title={t("chat.stillWorking")}
                      />
                    )}
                  </span>
                  <span className="text-xs opacity-70">
                    {conv.provider} · {conv.model}
                  </span>
                </button>
              )}
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem
                onClick={() => {
                  setRenamingId(conv.id)
                  setRenameValue(conv.title)
                }}
              >
                <PencilIcon />
                {t("chat.rename")}
              </ContextMenuItem>
              <ContextMenuItem onClick={() => handleArchiveToggle(conv)}>
                {conv.archived ? <ArchiveRestoreIcon /> : <ArchiveIcon />}
                {conv.archived ? t("chat.unarchive") : t("chat.archive")}
              </ContextMenuItem>
              <ContextMenuSeparator />
              <ContextMenuItem variant="destructive" onClick={() => handleDeleteConversation(conv)}>
                <Trash2Icon />
                {t("common.delete")}
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
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
              <ChangeModelDialog
                conversation={selectedConversation}
                onChanged={(updated) =>
                  setConversations((prev) => prev?.map((c) => (c.id === updated.id ? updated : c)) ?? prev)
                }
              />
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

              {currentStream.pendingContent && (
                <MessageBubble message={{ role: "user", content: currentStream.pendingContent }} />
              )}

              {currentStream.isStreaming && (
                <MessageBubble
                  message={{ role: "assistant", content: currentStream.streamingText || "…" }}
                  pending={currentStream.streamingText === ""}
                />
              )}

              {currentStream.toolCalls.map((tc) => (
                <LiveToolCallBubble key={tc.call.id} toolCall={tc} />
              ))}
            </div>

            <form onSubmit={handleSend} className="flex items-end gap-2 border-t p-3">
              <ComposerTextarea
                value={input}
                onChange={setInput}
                onSubmit={() => handleSend()}
                disabled={currentStream.isStreaming}
                placeholder={t("chat.composerPlaceholder")}
              />
              <Button type="submit" disabled={currentStream.isStreaming || !input.trim()}>
                {currentStream.isStreaming ? t("chat.sending") : t("chat.send")}
              </Button>
            </form>
          </>
        )}
      </section>
    </div>
  )
}

const MAX_COMPOSER_HEIGHT_PX = 160

// ComposerTextarea replaces a plain single-line <Input> — a chat message
// can legitimately be many lines (a pasted stack trace, a multi-line code
// block the user is asking about), which a single-line input can't hold
// at all: pressing Enter inside one submits the surrounding <form>
// immediately, so there's no way to type a second line to begin with.
// Enter alone still sends (matching the single-line composer's old
// behavior and every mainstream chat app); Shift+Enter inserts a literal
// newline. Grows with content up to MAX_COMPOSER_HEIGHT_PX, then scrolls
// internally rather than pushing the message list off-screen.
function ComposerTextarea({
  value,
  onChange,
  onSubmit,
  disabled,
  placeholder,
}: {
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  disabled?: boolean
  placeholder?: string
}) {
  const ref = useRef<HTMLTextAreaElement>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = "auto"
    el.style.height = `${Math.min(el.scrollHeight, MAX_COMPOSER_HEIGHT_PX)}px`
  }, [value])

  return (
    <textarea
      ref={ref}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={(e) => {
        // isComposing guards IME input (Chinese/Japanese/Korean, ...) —
        // Enter there confirms a character being composed, it isn't the
        // user asking to send yet.
        if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
          e.preventDefault()
          onSubmit()
        }
      }}
      disabled={disabled}
      placeholder={placeholder}
      rows={1}
      className="max-h-40 min-h-9 flex-1 resize-none overflow-y-auto rounded-lg border border-input bg-transparent px-3 py-2 text-sm transition-colors outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-input/30"
    />
  )
}

// Memoized so typing in the composer (which re-renders all of ChatPage on
// every keystroke) doesn't re-run every past message's markdown parse and
// syntax highlighting — the cost that made typing feel sluggish once a
// conversation had enough history. `message`/`pending` stay referentially
// stable across a typing-only re-render, so this bails out correctly.
const MessageBubble = memo(function MessageBubble({ message, pending }: { message: MessageDto; pending?: boolean }) {
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
          "max-w-[75%] min-w-0 rounded-lg px-3 py-2",
          isUser ? "bg-primary text-primary-foreground" : "bg-muted",
          pending && "text-muted-foreground italic",
        )}
      >
        <Markdown content={message.content ?? ""} />
      </div>
      {message.toolCalls?.map((call) => (
        <span key={call.id} className="text-xs text-muted-foreground">
          {t("chat.callingTool", { name: call.name, args: JSON.stringify(call.arguments) })}
        </span>
      ))}
    </div>
  )
})

// Shown from the moment a tool call is requested (spinner, no result yet)
// through to its result landing in place — the persisted equivalent
// (MessageBubble's "tool" role branch) only exists once the whole turn is
// already done, so this is what makes a call visible while it's running.
function LiveToolCallBubble({ toolCall }: { toolCall: LiveToolCall }) {
  const { t } = useLanguage()
  const { call, result } = toolCall

  return (
    <div className="max-w-[75%] space-y-1 rounded-md border bg-muted/50 px-3 py-2 text-xs">
      <div className="flex items-center gap-1.5">
        <span className="font-medium">{call.name}</span>
        {!result && <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-primary" />}
      </div>
      {!result && (
        <span className="text-muted-foreground">{t("chat.callingTool", { name: call.name, args: JSON.stringify(call.arguments) })}</span>
      )}
      {result?.error && <span className="text-destructive">{t("chat.toolError", { error: result.error })}</span>}
      {result && !result.error && (
        <pre className="mt-1 whitespace-pre-wrap break-words text-muted-foreground">
          {typeof result.result === "string" ? result.result : JSON.stringify(result.result, null, 2)}
        </pre>
      )}
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

// useModelSelection derives everything both NewConversationDialog and
// ChangeModelDialog need from a selectedKey — shared so the two pickers
// (creating a conversation vs. switching an existing one's model) can't
// drift out of sync on how a key resolves to an actual provider/model.
function useModelSelection(selectedKey: string, customModel: string, presets: CustomModelDto[]) {
  const selectedPreset = selectedKey.startsWith("custom:")
    ? presets.find((p) => customKey(p.id) === selectedKey)
    : undefined
  const builtinIndex = selectedKey.startsWith("builtin:") ? Number(selectedKey.slice("builtin:".length)) : -1
  const builtinOption = builtinIndex >= 0 ? MODEL_OPTIONS[builtinIndex] : undefined
  const isFreeformCustom = builtinOption?.provider === "openai" && builtinOption.model === ""

  const resolved = selectedPreset
    ? { provider: "openai", model: selectedPreset.modelName, customModelId: selectedPreset.id }
    : isFreeformCustom
      ? customModel.trim()
        ? { provider: "openai", model: customModel.trim(), customModelId: undefined }
        : null
      : builtinOption
        ? { provider: builtinOption.provider, model: builtinOption.model, customModelId: undefined }
        : null

  return { selectedPreset, builtinOption, isFreeformCustom, resolved }
}

function ModelSelectFields({
  selectedKey,
  onSelectedKeyChange,
  customModel,
  onCustomModelChange,
  presets,
  isFreeformCustom,
  selectedLabel,
}: {
  selectedKey: string
  onSelectedKeyChange: (key: string) => void
  customModel: string
  onCustomModelChange: (value: string) => void
  presets: CustomModelDto[]
  isFreeformCustom: boolean
  selectedLabel: string
}) {
  const { t } = useLanguage()
  return (
    <>
      <div className="space-y-2">
        <Label>{t("chat.modelLabel")}</Label>
        <Select value={selectedKey} onValueChange={(value) => onSelectedKeyChange(value ?? builtinKey(0))}>
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
          <Label htmlFor="model-custom-name">{t("chat.modelNameLabel")}</Label>
          <Input
            id="model-custom-name"
            value={customModel}
            onChange={(e) => onCustomModelChange(e.target.value)}
            placeholder={t("chat.modelNamePlaceholder")}
            required
          />
        </div>
      )}
    </>
  )
}

function NewConversationDialog({ onCreated }: { onCreated: (conv: ConversationDto) => void }) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
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

  const { selectedPreset, builtinOption, isFreeformCustom, resolved } = useModelSelection(
    selectedKey,
    customModel,
    presets,
  )
  const selectedLabel = selectedPreset?.name ?? builtinOption?.label ?? t("chat.modelLabel")

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!resolved) return

    setLoading(true)
    try {
      const conv = await api.conversations.create(resolved)
      toast.success(t("chat.conversationCreated"))
      setOpen(false)
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
            <ModelSelectFields
              selectedKey={selectedKey}
              onSelectedKeyChange={setSelectedKey}
              customModel={customModel}
              onCustomModelChange={setCustomModel}
              presets={presets}
              isFreeformCustom={isFreeformCustom}
              selectedLabel={selectedLabel}
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !resolved}>
              {loading ? t("common.creating") : t("common.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// ChangeModelDialog switches which model an *existing* conversation talks
// to going forward — history stays exactly as-is regardless of which
// model produced which message; only the next turn uses the new one.
function ChangeModelDialog({
  conversation,
  onChanged,
}: {
  conversation: ConversationDto
  onChanged: (conv: ConversationDto) => void
}) {
  const { t } = useLanguage()
  const [open, setOpen] = useState(false)
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

  // Seed the picker from the conversation's current model every time the
  // dialog opens — matching one of MODEL_OPTIONS by provider+model, one of
  // the presets by customModelId, or falling back to the freeform custom
  // slot for an openai model that matches neither.
  useEffect(() => {
    if (!open) return
    if (conversation.customModelId) {
      setSelectedKey(customKey(conversation.customModelId))
      return
    }
    const builtinIndex = MODEL_OPTIONS.findIndex(
      (o) => o.provider === conversation.provider && o.model === conversation.model,
    )
    if (builtinIndex >= 0) {
      setSelectedKey(builtinKey(builtinIndex))
    } else {
      setSelectedKey(builtinKey(MODEL_OPTIONS.length - 1))
      setCustomModel(conversation.model)
    }
    // conversation intentionally excluded beyond the fields read above —
    // this only needs to reseed when the dialog (re)opens or the
    // conversation it's showing changes, not on every unrelated re-render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, conversation.id, conversation.provider, conversation.model, conversation.customModelId])

  const { selectedPreset, builtinOption, isFreeformCustom, resolved } = useModelSelection(
    selectedKey,
    customModel,
    presets,
  )
  const selectedLabel = selectedPreset?.name ?? builtinOption?.label ?? t("chat.modelLabel")

  const isUnchanged =
    resolved !== null &&
    resolved.provider === conversation.provider &&
    resolved.model === conversation.model &&
    (resolved.customModelId ?? null) === (conversation.customModelId ?? null)

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!resolved) return

    setLoading(true)
    try {
      const updated = await api.conversations.update(conversation.id, resolved)
      toast.success(t("chat.modelChanged"))
      setOpen(false)
      onChanged(updated)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t("chat.modelChangeFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <button className="inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs text-muted-foreground hover:border-foreground/30 hover:text-foreground">
            {conversation.provider} · {conversation.model}
            <ChevronDownIcon className="size-3" />
          </button>
        }
      />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("chat.changeModelDialogTitle")}</DialogTitle>
            <DialogDescription>{t("chat.changeModelDialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <ModelSelectFields
              selectedKey={selectedKey}
              onSelectedKeyChange={setSelectedKey}
              customModel={customModel}
              onCustomModelChange={setCustomModel}
              presets={presets}
              isFreeformCustom={isFreeformCustom}
              selectedLabel={selectedLabel}
            />
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || !resolved || isUnchanged}>
              {loading ? t("chat.switchingModel") : t("chat.switchModel")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
