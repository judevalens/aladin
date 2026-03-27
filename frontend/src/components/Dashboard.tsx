import { useState, useEffect, useRef } from "react";
import type { Artifact, ArtifactType } from "../types";
import AudioRecorder from "./AudioRecorder";
import { ingestUrl, streamChat, fetchChildren } from "../lib/api";

// ── Constants ─────────────────────────────────────────────────────────────────

const TYPE_ICON: Record<ArtifactType, string> = {
  audio: "🎙", link: "🔗", text: "📋", file: "📎", feed: "📡", post: "💬",
};
const TYPE_LABEL: Record<ArtifactType, string> = {
  audio: "Audio", link: "Link", text: "Note", file: "File", feed: "Feed", post: "Post",
};

interface Message { id: number; role: "user" | "assistant"; content: string }
let msgId = 1;

interface Props {
  artifacts: Artifact[];
  onAddArtifact: (type: ArtifactType, label: string, content: string, sourceUrl?: string) => void;
  onAddToGraph: (artifact: Artifact) => void;
  onOpenWorkspace: () => void;
}

type View = "grid" | "feed" | "focus";

// ── Dashboard ─────────────────────────────────────────────────────────────────

export default function Dashboard({ artifacts, onAddArtifact, onAddToGraph, onOpenWorkspace }: Props) {
  const [view, setView]                 = useState<View>("grid");
  const [focused, setFocused]           = useState<Artifact | null>(null);
  const [feedArtifact, setFeedArtifact] = useState<Artifact | null>(null);
  const [filter, setFilter]             = useState<ArtifactType | "all">("all");
  const [search, setSearch]             = useState("");
  const [showRecorder, setShowRecorder] = useState(false);

  const openFocus = (a: Artifact) => { setFocused(a); setView("focus"); };
  const openFeed  = (a: Artifact) => { setFeedArtifact(a); setView("feed"); };

  const visible = artifacts.filter((a) => {
    if (filter !== "all" && a.type !== filter) return false;
    if (search && !a.label.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const handleLink = async () => {
    const url = prompt("Paste a URL:");
    if (!url) return;
    try {
      const result = await ingestUrl(url);
      if (result.post) {
        const { authorName, authorHandle, content, platform } = result.post;
        onAddArtifact("link", `${authorName} (@${authorHandle}) · ${platform}`, content, url);
        return;
      }
    } catch { /* fall through */ }
    onAddArtifact("link", url.replace(/^https?:\/\//, "").split("/")[0], url, url);
  };

  const handlePaste = async () => {
    const text = await navigator.clipboard.readText();
    if (text) onAddArtifact("text", text.slice(0, 40) + (text.length > 40 ? "…" : ""), text);
  };

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) onAddArtifact("file", file.name, file.name);
    e.target.value = "";
  };

  if (view === "feed" && feedArtifact) {
    return (
      <FeedView
        feed={feedArtifact}
        onBack={() => setView("grid")}
        onOpenPost={(post) => { setFocused(post); setView("focus"); }}
        onOpenWorkspace={onOpenWorkspace}
      />
    );
  }

  if (view === "focus" && focused) {
    return (
      <FocusView
        artifact={focused}
        artifacts={artifacts}
        onBack={() => focused.type === "post" && feedArtifact ? setView("feed") : setView("grid")}
        onNavigate={openFocus}
        onAddToGraph={() => { onAddToGraph(focused); onOpenWorkspace(); }}
        onOpenWorkspace={onOpenWorkspace}
      />
    );
  }

  return (
    <div className="flex flex-col h-full w-full bg-gray-50">

      {/* Top bar */}
      <div className="bg-white border-b border-gray-100 px-10 py-6 shrink-0">
        <div className="flex items-center justify-between mb-5">
          <h1 className="text-lg font-semibold text-gray-900">Research</h1>
          <button
            onClick={onOpenWorkspace}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-gray-200 text-xs text-gray-500 hover:bg-gray-50 transition-colors"
          >
            ⬡ Graph
          </button>
        </div>

        <div className="relative max-w-2xl mb-4">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">⌕</span>
          <input
            type="text"
            placeholder="Search artifacts..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-8 pr-4 py-2.5 rounded-lg border border-gray-200 bg-gray-50 text-sm outline-none focus:border-gray-400 focus:bg-white transition-colors"
          />
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-xs text-gray-400">Add</span>
          <button onClick={() => setShowRecorder(true)} className="quick-add-btn">🎙 Record</button>
          <button onClick={handleLink} className="quick-add-btn">🔗 Link</button>
          <button onClick={handlePaste} className="quick-add-btn">📋 Paste</button>
          <label className="quick-add-btn cursor-pointer">
            📎 File <input type="file" className="hidden" onChange={handleFile} />
          </label>
          <div className="ml-auto flex gap-1">
            {(["all", "feed", "file", "link", "text", "audio"] as const).map((t) => (
              <button
                key={t}
                onClick={() => setFilter(t)}
                className={`px-3 py-1 rounded-full text-xs font-medium transition-colors ${
                  filter === t ? "bg-gray-900 text-white" : "text-gray-500 hover:text-gray-800 hover:bg-gray-100"
                }`}
              >
                {t === "all" ? "All" : TYPE_LABEL[t as ArtifactType]}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Grid */}
      <div className="flex-1 overflow-y-auto px-10 py-6">
        {visible.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-300 gap-3">
            <span className="text-5xl">◻</span>
            <p className="text-sm">No artifacts yet — add something above</p>
          </div>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-4">
            {visible.map((a) => (
              <ArtifactCard
                key={a.id}
                artifact={a}
                onClick={() => a.type === "feed" ? openFeed(a) : openFocus(a)}
              />
            ))}
          </div>
        )}
      </div>

      {showRecorder && (
        <AudioRecorder
          onSave={(filename, label) => { onAddArtifact("audio", label, `/api/audio/${filename}`); setShowRecorder(false); }}
          onCancel={() => setShowRecorder(false)}
        />
      )}
    </div>
  );
}

// ── Feed drill-down ────────────────────────────────────────────────────────────

function FeedView({ feed, onBack, onOpenPost, onOpenWorkspace }: {
  feed: Artifact;
  onBack: () => void;
  onOpenPost: (post: Artifact) => void;
  onOpenWorkspace: () => void;
}) {
  const [posts, setPosts]     = useState<Artifact[]>([]);
  const [total, setTotal]     = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch]   = useState("");
  const PAGE = 50;

  useEffect(() => {
    setLoading(true);
    fetchChildren(feed.id, { limit: PAGE })
      .then(({ items, total }) => { setPosts(items); setTotal(total); })
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [feed.id]);

  const subreddit = (feed.metadata?.subreddit as string) ?? feed.label;

  const visible = search
    ? posts.filter((p) => p.label.toLowerCase().includes(search.toLowerCase()))
    : posts;

  return (
    <div className="flex flex-col h-full w-full bg-gray-50">

      {/* Header */}
      <div className="bg-white border-b border-gray-100 px-10 py-5 shrink-0">
        <div className="flex items-center gap-3 mb-4">
          <button
            onClick={onBack}
            className="text-xs text-gray-400 hover:text-gray-700 transition-colors"
          >
            ← Research
          </button>
          <span className="text-gray-200">/</span>
          <span className="text-sm font-medium text-gray-900">{feed.label}</span>
          <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-600 text-[10px] font-medium uppercase tracking-wide">
            Live
          </span>
          <span className="text-xs text-gray-400 ml-auto">{total} posts</span>
          <button
            onClick={onOpenWorkspace}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-gray-200 text-xs text-gray-500 hover:bg-gray-50 transition-colors"
          >
            ⬡ Graph
          </button>
        </div>

        <div className="relative max-w-xl">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">⌕</span>
          <input
            type="text"
            placeholder={`Search in ${subreddit}…`}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-8 pr-4 py-2 rounded-lg border border-gray-200 bg-gray-50 text-sm outline-none focus:border-gray-400 focus:bg-white transition-colors"
          />
        </div>
      </div>

      {/* Post list */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-40 text-gray-300 text-sm">Loading…</div>
        ) : visible.length === 0 ? (
          <div className="flex items-center justify-center h-40 text-gray-300 text-sm">No posts found</div>
        ) : (
          <div className="divide-y divide-gray-100">
            {visible.map((post) => (
              <PostRow key={post.id} post={post} onClick={() => onOpenPost(post)} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function PostRow({ post, onClick }: { post: Artifact; onClick: () => void }) {
  const score    = (post.metadata?.score as number) ?? 0;
  const comments = (post.metadata?.num_comments as number) ?? 0;
  const author   = (post.metadata?.author as string) ?? "";
  const flair    = post.metadata?.flair as string | null;

  return (
    <button
      onClick={onClick}
      className="w-full text-left px-10 py-4 hover:bg-white transition-colors group"
    >
      <div className="flex items-start gap-4">
        {/* Score */}
        <div className="w-10 shrink-0 text-center">
          <span className="text-sm font-medium text-gray-500">{score >= 1000 ? `${(score / 1000).toFixed(1)}k` : score}</span>
          <p className="text-[10px] text-gray-300">pts</p>
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            {flair && (
              <span className="px-1.5 py-0.5 rounded bg-gray-100 text-[10px] text-gray-500 shrink-0">{flair}</span>
            )}
            <p className="text-sm font-medium text-gray-900 group-hover:text-black leading-snug line-clamp-2">
              {post.label}
            </p>
          </div>
          <div className="flex items-center gap-3 text-[11px] text-gray-400">
            {author && <span>u/{author}</span>}
            <span>💬 {comments}</span>
            <span>{new Date(post.createdAt).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</span>
          </div>
        </div>

        {/* Arrow */}
        <span className="text-gray-200 group-hover:text-gray-400 transition-colors shrink-0">→</span>
      </div>
    </button>
  );
}

// ── Focus view ────────────────────────────────────────────────────────────────

function FocusView({ artifact, artifacts, onBack, onAddToGraph }: {
  artifact: Artifact;
  artifacts: Artifact[];
  onBack: () => void;
  onNavigate?: (a: Artifact) => void;
  onAddToGraph: () => void;
  onOpenWorkspace?: () => void;
}) {
  const [context, setContext]         = useState<Artifact[]>([artifact]);
  const [preview, setPreview]         = useState<Artifact>(artifact);
  const [messages, setMessages]       = useState<Message[]>([]);
  const [input, setInput]             = useState("");
  const [loading, setLoading]         = useState(false);
  const [showLibrary, setShowLibrary] = useState(false);
  const [libSearch, setLibSearch]     = useState("");
  const bottomRef                     = useRef<HTMLDivElement>(null);
  const inputRef                      = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    setPreview(artifact);
    setMessages([]);
  }, [artifact.id]);

  useEffect(() => { bottomRef.current?.scrollIntoView({ behavior: "smooth" }); }, [messages]);

  const related = artifacts.filter((a) => {
    if (a.id === preview.id || !preview.enrichment || !a.enrichment) return false;
    const myTopics = new Set(preview.enrichment.topics);
    return a.enrichment.topics.some((t) => myTopics.has(t));
  }).slice(0, 6);

  const toggleContext = (a: Artifact) => {
    setContext((prev) =>
      prev.find((x) => x.id === a.id) ? prev.filter((x) => x.id !== a.id) : [...prev, a]
    );
  };

  const send = async () => {
    const text = input.trim();
    if (!text || loading) return;

    const contextBlock = context.map((a) => `# ${a.label}\n${a.content}`).join("\n\n");
    const userMsg: Message = { id: msgId++, role: "user", content: text };
    setMessages((prev) => [...prev, userMsg]);
    setInput("");
    setLoading(true);
    const assistantId = msgId++;
    setMessages((prev) => [...prev, { id: assistantId, role: "assistant", content: "" }]);
    try {
      const history = [...messages, userMsg].map((m) => ({ role: m.role, content: m.content }));
      for await (const token of streamChat(`${contextBlock}\n\n${text}`, history)) {
        setMessages((prev) => prev.map((m) => m.id === assistantId ? { ...m, content: m.content + token } : m));
      }
    } catch {
      setMessages((prev) => prev.map((m) => m.id === assistantId ? { ...m, content: "Something went wrong." } : m));
    } finally {
      setLoading(false);
    }
  };

  const icon  = TYPE_ICON[preview.type]  ?? "◻";
  const label = TYPE_LABEL[preview.type] ?? preview.type;

  return (
    <div className="flex flex-col h-full w-full bg-white">

      {/* Header */}
      <div className="flex items-center gap-3 px-6 py-3 border-b border-gray-100 shrink-0">
        <button
          onClick={onBack}
          className="flex items-center gap-1.5 text-xs text-gray-400 hover:text-gray-700 transition-colors"
        >
          ←
        </button>
        <span className="text-gray-200">/</span>
        <span className="text-xs text-gray-700 font-medium truncate max-w-xs">{artifact.label}</span>
        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={onAddToGraph}
            className="px-3 py-1.5 rounded-md border border-gray-200 text-xs text-gray-600 hover:bg-gray-50 transition-colors"
          >
            ⬡ Add to graph
          </button>
          {artifact.sourceUrl && (
            <a
              href={artifact.sourceUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="px-3 py-1.5 rounded-md border border-gray-200 text-xs text-gray-600 hover:bg-gray-50 transition-colors"
            >
              ↗ Source
            </a>
          )}
        </div>
      </div>

      {/* Body: detail + chat */}
      <div className="flex flex-1 min-h-0">

        {/* Left: artifact detail */}
        <div className="w-96 shrink-0 border-r border-gray-100 flex flex-col overflow-y-auto">
          <div className="px-7 py-6 space-y-5">
            <div>
              <div className="flex items-center gap-2 mb-3">
                <span className="text-lg">{icon}</span>
                <span className="text-[10px] text-gray-400 uppercase tracking-widest">{label}</span>
                {preview.metadata?.score !== undefined && (
                  <span className="ml-auto text-xs text-gray-400">
                    ▲ {preview.metadata.score as number}
                  </span>
                )}
              </div>
              <h2 className="text-base font-semibold text-gray-900 leading-snug">{preview.label}</h2>
            </div>

            {preview.enrichment?.summary && (
              <div>
                <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Summary</p>
                <p className="text-sm text-gray-600 leading-relaxed">{preview.enrichment.summary}</p>
              </div>
            )}

            {preview.enrichment?.topics?.length ? (
              <div>
                <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Topics</p>
                <div className="flex flex-wrap gap-1.5">
                  {preview.enrichment.topics.map((t) => (
                    <span key={t} className="px-2.5 py-1 rounded-full bg-gray-100 text-xs text-gray-600">{t}</span>
                  ))}
                </div>
              </div>
            ) : null}

            {preview.enrichment?.key_claims?.length ? (
              <div>
                <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Key claims</p>
                <ul className="space-y-2">
                  {preview.enrichment.key_claims.map((c, i) => (
                    <li key={i} className="text-sm text-gray-600 leading-relaxed flex gap-2">
                      <span className="text-gray-300 shrink-0">—</span>{c}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}

            <div>
              <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Content</p>
              <p className="text-sm text-gray-500 leading-relaxed whitespace-pre-wrap">{preview.content}</p>
            </div>

            {related.length > 0 && (
              <div>
                <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Related</p>
                <div className="space-y-0.5">
                  {related.map((a) => (
                    <div key={a.id} className="flex items-center gap-2 group">
                      <button
                        onClick={() => setPreview(a)}
                        className="flex items-center gap-2 flex-1 py-2 px-2 rounded-lg hover:bg-gray-50 text-left transition-colors"
                      >
                        <span className="text-sm shrink-0">{TYPE_ICON[a.type] ?? "◻"}</span>
                        <span className="text-sm text-gray-700 truncate">{a.label}</span>
                      </button>
                      <button
                        onClick={() => toggleContext(a)}
                        title={context.find((x) => x.id === a.id) ? "Remove from context" : "Add to chat context"}
                        className={`text-xs px-2 py-1 rounded-md transition-colors shrink-0 ${
                          context.find((x) => x.id === a.id)
                            ? "bg-gray-900 text-white"
                            : "text-gray-300 hover:text-gray-600 hover:bg-gray-100"
                        }`}
                      >
                        +
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Right: chat */}
        <div className="flex-1 flex flex-col min-w-0">

          <div className="flex items-center gap-2 px-6 py-2.5 border-b border-gray-100 bg-gray-50 shrink-0 flex-wrap">
            <span className="text-[10px] text-gray-400 uppercase tracking-widest shrink-0">Context</span>
            {context.map((a) => (
              <span
                key={a.id}
                className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-white border border-gray-200 text-xs text-gray-600"
              >
                {TYPE_ICON[a.type] ?? "◻"} {a.label.slice(0, 30)}{a.label.length > 30 ? "…" : ""}
                <button onClick={() => toggleContext(a)} className="text-gray-300 hover:text-gray-600 leading-none ml-0.5">×</button>
              </span>
            ))}
            <button
              onClick={() => setShowLibrary((v) => !v)}
              className={`ml-auto flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs transition-colors ${
                showLibrary ? "bg-gray-900 text-white" : "text-gray-400 hover:text-gray-700 hover:bg-gray-100"
              }`}
            >
              ☰ Library
            </button>
          </div>

          <div className="flex-1 overflow-y-auto px-6 py-5 space-y-4">
            {messages.length === 0 && (
              <div className="flex flex-col items-center justify-center h-full text-gray-300 gap-2">
                <p className="text-sm">Ask anything about {artifact.label}</p>
                <p className="text-xs">Use + on related items to add more context</p>
              </div>
            )}
            {messages.map((msg) => (
              <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[80%] rounded-2xl px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap ${
                  msg.role === "user"
                    ? "bg-gray-900 text-white rounded-br-sm"
                    : "bg-gray-100 text-gray-800 rounded-bl-sm"
                }`}>
                  {msg.content}
                  {msg.role === "assistant" && msg.content === "" && loading && (
                    <span className="flex gap-1 items-center h-4">
                      {[0, 1, 2].map((i) => (
                        <span key={i} className="w-1.5 h-1.5 rounded-full bg-gray-400 animate-bounce" style={{ animationDelay: `${i * 0.15}s` }} />
                      ))}
                    </span>
                  )}
                </div>
              </div>
            ))}
            <div ref={bottomRef} />
          </div>

          <div className="border-t border-gray-100 px-6 py-4 shrink-0">
            <textarea
              ref={inputRef}
              rows={3}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); send(); } }}
              placeholder={`Ask about ${artifact.label}…`}
              className="w-full resize-none rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-900 placeholder-gray-400 focus:outline-none focus:border-gray-300 transition-colors"
            />
            <div className="flex justify-end mt-2">
              <button
                onClick={send}
                disabled={!input.trim() || loading}
                className="px-5 py-2 rounded-lg bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 disabled:opacity-30 transition-colors"
              >
                Send
              </button>
            </div>
          </div>
        </div>

        {showLibrary && (
          <div className="w-64 shrink-0 border-l border-gray-100 flex flex-col bg-gray-50">
            <div className="px-4 py-3 border-b border-gray-100 shrink-0">
              <input
                type="text"
                placeholder="Filter…"
                value={libSearch}
                onChange={(e) => setLibSearch(e.target.value)}
                className="w-full px-3 py-1.5 rounded-lg border border-gray-200 bg-white text-xs outline-none focus:border-gray-300"
              />
            </div>
            <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
              {artifacts
                .filter((a) => !libSearch || a.label.toLowerCase().includes(libSearch.toLowerCase()))
                .map((a) => {
                  const inContext = !!context.find((x) => x.id === a.id);
                  return (
                    <div key={a.id} className="flex items-center gap-2 px-2 py-2 rounded-lg hover:bg-white transition-colors group">
                      <span className="text-sm shrink-0">{TYPE_ICON[a.type] ?? "◻"}</span>
                      <button
                        onClick={() => setPreview(a)}
                        className="flex-1 text-xs text-gray-700 truncate text-left"
                      >
                        {a.label}
                      </button>
                      <button
                        onClick={() => toggleContext(a)}
                        className={`shrink-0 w-5 h-5 rounded flex items-center justify-center text-xs transition-colors ${
                          inContext ? "bg-gray-900 text-white" : "text-gray-300 hover:text-gray-600 hover:bg-gray-200"
                        }`}
                      >
                        {inContext ? "✓" : "+"}
                      </button>
                    </div>
                  );
                })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// ── Artifact card ─────────────────────────────────────────────────────────────

function ArtifactCard({ artifact, onClick }: { artifact: Artifact; onClick: () => void }) {
  const isFeed = artifact.type === "feed";

  return (
    <button
      onClick={onClick}
      className="text-left rounded-xl border border-gray-200 p-5 bg-white hover:border-gray-300 hover:shadow-sm transition-all"
    >
      <div className="flex items-center justify-between mb-4">
        <span className="text-xl">{TYPE_ICON[artifact.type] ?? "◻"}</span>
        <div className="flex items-center gap-2">
          {isFeed && (
            <span className="px-2 py-0.5 rounded-full bg-emerald-50 text-emerald-600 text-[10px] font-medium uppercase tracking-wide">
              Live
            </span>
          )}
          <span className="text-[10px] text-gray-400 uppercase tracking-widest">
            {TYPE_LABEL[artifact.type] ?? artifact.type}
          </span>
        </div>
      </div>

      <p className="text-sm font-medium text-gray-900 leading-snug line-clamp-2 mb-2">{artifact.label}</p>

      {isFeed ? (
        <p className="text-xs text-gray-400">
          {artifact.childCount ?? 0} posts · click to explore
        </p>
      ) : artifact.enrichment?.summary ? (
        <p className="text-xs text-gray-400 line-clamp-3 leading-relaxed">{artifact.enrichment.summary}</p>
      ) : (
        <p className="text-xs text-gray-300 line-clamp-3 leading-relaxed">{artifact.content}</p>
      )}

      {!isFeed && artifact.enrichment?.topics?.length ? (
        <div className="flex flex-wrap gap-1 mt-3">
          {artifact.enrichment.topics.slice(0, 3).map((t) => (
            <span key={t} className="px-1.5 py-0.5 rounded bg-gray-100 text-[10px] text-gray-500">{t}</span>
          ))}
        </div>
      ) : null}

      <p className="mt-4 text-[10px] text-gray-300">
        {new Date(artifact.createdAt).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}
      </p>
    </button>
  );
}
