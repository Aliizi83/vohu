import { useCallback, useEffect, useRef, useState, type FormEvent } from "react"
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
import {
  api,
  ApiError,
  streamMessage,
  type ConversationDto,
  type MessageDto,
  type SSHConnectionDto,
} from "@/lib/api"
import { cn } from "cn"

const MODEL_OPTIONS = [
  { label: "Gemini Flash", provider: "gemini", model: "gemini-3.7-flash" },
  { label: "Gemini Flash Lite 3.5", provider: "gemini", model: "gemini-3.5-flash-lite" },
  { label: "Claude Opus 5", provider: "anthropic", model: "claude-opus-5" },
  { label: "Claude Sonnet 5", provider: "anthropic", model: "claude-sonnet-5" },
  { label: "Claude Haiku 4.5", provider: "anthropic", model: "claude-haiku-4-5-20251001" },
  { label: "Custom (OpenAI-compatible)", provider: "openai", model: "" },
] as const

export default function ChatPage() {
  const [conversations, setConversations] = useState<ConversationDto[] | null>(null)
  const [connections, setConnections] = useState<SSHConnectionDto[]>([])
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [messages, setMessages] = useState<MessageDto[] | null>(null)

  const [pendingUserContent, setPendingUserContent] = useState<string | null>(null)
  const [streamingText, setStreamingText] = useState("")
  const [isStreaming, setIsStreaming] = useState(false)
  const [input, setInput] = useState("")

  const scrollRef = useRef<HTMLDivElement>(null)

  const loadConversations = useCallback(async () => {
    try {
      const page = await api.conversations.list(1, 50)
      setConversations(page.items)
      if (page.items.length > 0 && selectedId === null) {
        setSelectedId(page.items[0].id)
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load conversations")
    }
    // selectedId intentionally excluded — this only picks a default once.
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
      return
    }
    setMessages(null)
    api.conversations
      .messages(selectedId)
      .then(setMessages)
      .catch((err) => {
        toast.error(err instanceof ApiError ? err.message : "Failed to load messages")
        setMessages([])
      })
  }, [selectedId])

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [messages, streamingText, pendingUserContent])

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
          <h2 className="text-sm font-semibold text-muted-foreground">Conversations</h2>
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
          <p className="px-1 py-4 text-sm text-muted-foreground">No conversations yet.</p>
        )}

        {conversations?.map((conv) => (
          <button
            key={conv.id}
            onClick={() => setSelectedId(conv.id)}
            className={cn(
              "flex flex-col items-start rounded-md px-3 py-2 text-left text-sm transition-colors",
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
            <h3 className="mb-2 text-xs font-semibold text-muted-foreground">SSH connections</h3>
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
            {conversations === null ? "Loading..." : "Create or select a conversation to start chatting."}
          </div>
        ) : (
          <>
            <div className="border-b px-4 py-3">
              <h2 className="font-semibold">{selectedConversation.title}</h2>
              <p className="text-xs text-muted-foreground">
                {selectedConversation.provider} · {selectedConversation.model}
              </p>
            </div>

            <div ref={scrollRef} className="flex-1 space-y-3 overflow-y-auto px-4 py-4">
              {messages === null &&
                Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-12 w-2/3" />)}

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
                placeholder="Ask the agent to do something, e.g. run `pwd` on one of your servers…"
                disabled={isStreaming}
              />
              <Button type="submit" disabled={isStreaming || !input.trim()}>
                {isStreaming ? "Sending…" : "Send"}
              </Button>
            </form>
          </>
        )}
      </section>
    </div>
  )
}

function MessageBubble({ message, pending }: { message: MessageDto; pending?: boolean }) {
  if (message.role === "tool") {
    return (
      <div className="space-y-1 rounded-md border bg-muted/50 px-3 py-2 text-xs">
        {message.toolResults?.map((result, i) => (
          <div key={i}>
            <span className="font-medium">{result.name}</span>{" "}
            {result.error ? (
              <span className="text-destructive">error: {result.error}</span>
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
          → calling {call.name}({JSON.stringify(call.arguments)})
        </span>
      ))}
    </div>
  )
}

function NewConversationDialog({ onCreated }: { onCreated: (conv: ConversationDto) => void }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState("")
  const [optionIndex, setOptionIndex] = useState("0")
  const [customModel, setCustomModel] = useState("")
  const [loading, setLoading] = useState(false)

  const option = MODEL_OPTIONS[Number(optionIndex)]
  const isCustom = option.provider === "openai" && option.model === ""

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    const model = isCustom ? customModel.trim() : option.model
    if (!model) return

    setLoading(true)
    try {
      const conv = await api.conversations.create({
        title: title.trim() || "New chat",
        provider: option.provider,
        model,
      })
      toast.success("Conversation created")
      setOpen(false)
      setTitle("")
      setCustomModel("")
      onCreated(conv)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create conversation")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm">New chat</Button>} />
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>New conversation</DialogTitle>
            <DialogDescription>
              Provider and model are fixed for the whole conversation once created.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="conv-title">Title</Label>
              <Input
                id="conv-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="New chat"
              />
            </div>
            <div className="space-y-2">
              <Label>Model</Label>
              <Select value={optionIndex} onValueChange={(value) => setOptionIndex(value ?? "0")}>
                <SelectTrigger className="w-full">
                  <SelectValue>
                    {(value: string) => MODEL_OPTIONS[Number(value)]?.label ?? "Choose a model"}
                  </SelectValue>
                </SelectTrigger>
                <SelectContent>
                  {MODEL_OPTIONS.map((opt, i) => (
                    <SelectItem key={opt.label} value={String(i)}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {isCustom && (
              <div className="space-y-2">
                <Label htmlFor="conv-custom-model">Model name</Label>
                <Input
                  id="conv-custom-model"
                  value={customModel}
                  onChange={(e) => setCustomModel(e.target.value)}
                  placeholder="e.g. gpt-4o, deepseek-chat"
                  required
                />
              </div>
            )}
          </div>
          <DialogFooter>
            <Button type="submit" disabled={loading || (isCustom && !customModel.trim())}>
              {loading ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
