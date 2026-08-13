import { Activity, BarChart3, CornerDownRight, FileText, Layers } from "lucide-react";
import { memo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Icon } from "@/components/ui/icon";
import { useCitationNav } from "@/modules/copilot/hooks/use-citation-nav";
import { cn } from "@/lib/utils";

type MarkdownSegment =
  | { kind: "markdown"; text: string }
  | { kind: "directive"; name: string; attrs: Record<string, string>; body: string };

/**
 * Renders assistant markdown with Aladin tokens. react-markdown does not render raw HTML by
 * default (no rehype-raw), so this is safe against injected markup. Kept compact for the dock.
 */
export const CopilotMarkdown = memo(function CopilotMarkdown({
  text,
  onPrompt,
}: {
  text: string;
  onPrompt?: (prompt: string) => void;
}) {
  const navCitation = useCitationNav();
  const segments = parseCopilotMarkdown(text);
  return (
    <div className="text-body leading-relaxed text-ink-2">
      {segments.map((segment, index) =>
        segment.kind === "markdown" ? (
          <MarkdownText key={index} text={segment.text} />
        ) : (
          <DirectiveBlock
            key={index}
            segment={segment}
            onNavigate={(kind, id, title) => navCitation({ kind, id, title })}
            onPrompt={onPrompt}
          />
        ),
      )}
    </div>
  );
});

function MarkdownText({ text }: { text: string }) {
  if (!text.trim()) return null;
  return (
    <Markdown
      remarkPlugins={[remarkGfm]}
      components={{
        p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
        strong: ({ children }) => <strong className="font-semibold text-ink">{children}</strong>,
        em: ({ children }) => <em className="italic">{children}</em>,
        a: ({ href, children }) => (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-amber underline decoration-amber-line underline-offset-2 hover:opacity-80"
          >
            {children}
          </a>
        ),
        ul: ({ children }) => <ul className="mb-2 ml-4 list-disc space-y-1 last:mb-0">{children}</ul>,
        ol: ({ children }) => <ol className="mb-2 ml-4 list-decimal space-y-1 last:mb-0">{children}</ol>,
        li: ({ children }) => <li className="marker:text-ink-4">{children}</li>,
        h1: ({ children }) => <h1 className="mb-1.5 mt-1 font-display text-lead font-semibold text-ink">{children}</h1>,
        h2: ({ children }) => <h2 className="mb-1.5 mt-1 font-display text-lead font-semibold text-ink">{children}</h2>,
        h3: ({ children }) => <h3 className="mb-1 mt-1 text-body font-semibold text-ink">{children}</h3>,
        blockquote: ({ children }) => (
          <blockquote className="my-2 border-l-2 border-amber-line pl-2.5 text-ink-3">{children}</blockquote>
        ),
        hr: () => <hr className="my-3 border-line-2" />,
        code: ({ className, children }) => {
          const isBlock = /language-/.test(className ?? "");
          if (isBlock) {
            return <code className="font-mono text-small text-ink">{children}</code>;
          }
          return (
            <code className="rounded-tap bg-raise px-1 py-0.5 font-mono text-small text-ink">{children}</code>
          );
        },
        pre: ({ children }) => (
          <pre className="my-2 overflow-x-auto rounded-card border border-line bg-field p-2.5">{children}</pre>
        ),
        table: ({ children }) => (
          <div className="my-2 overflow-x-auto">
            <table className="w-full border-collapse text-small">{children}</table>
          </div>
        ),
        th: ({ children }) => <th className="border border-line px-2 py-1 text-left font-semibold text-ink">{children}</th>,
        td: ({ children }) => <td className="border border-line px-2 py-1 text-ink-2">{children}</td>,
      }}
    >
      {text}
    </Markdown>
  );
}

function DirectiveBlock({
  segment,
  onNavigate,
  onPrompt,
}: {
  segment: Extract<MarkdownSegment, { kind: "directive" }>;
  onNavigate: (kind: string, id: string, title: string) => void;
  onPrompt?: (prompt: string) => void;
}) {
  switch (segment.name) {
    case "aladin-artifact": {
      const id = segment.attrs.id;
      const title = segment.attrs.title || id;
      const kind = segment.attrs.kind || "artifact";
      if (!id || !title) return <MarkdownText text={directiveFallback(segment)} />;
      const citationKind = kind === "app" ? "shard" : kind;
      return (
        <button
          type="button"
          onClick={() => onNavigate(citationKind, id, title)}
          className="my-2 flex w-full items-center gap-2 rounded-card border border-line bg-raise px-2.5 py-2 text-left transition-colors hover:border-amber-line"
        >
          <Icon as={citationKind === "shard" ? Layers : FileText} mark className="shrink-0 text-amber" />
          <span className="min-w-0">
            <span className="block truncate text-small font-semibold text-ink">{title}</span>
            <span className="block font-mono text-meta text-ink-4">{citationKind}</span>
          </span>
        </button>
      );
    }
    case "aladin-ticker": {
      const symbol = segment.attrs.symbol?.toUpperCase();
      if (!symbol) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <button
          type="button"
          onClick={() => onNavigate("ticker", symbol, symbol)}
          className="my-2 flex w-full items-center justify-between rounded-card border border-line bg-field px-2.5 py-2 text-left transition-colors hover:border-amber-line"
        >
          <span className="flex items-center gap-2">
            <Icon as={BarChart3} mark className="text-amber" />
            <span className="font-mono text-small font-semibold text-ink">{symbol}</span>
          </span>
          <span className="font-mono text-meta text-ink-4">ticker</span>
        </button>
      );
    }
    case "aladin-activity": {
      const items = parseActivityItems(segment.body);
      if (items.length === 0) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <div className="my-2 rounded-card border border-line bg-field px-2.5 py-2">
          <div className="mb-1.5 flex items-center gap-1.5 font-mono text-meta text-ink-4">
            <Icon as={Activity} size="inline" mark />
            activity
          </div>
          <div className="space-y-1">
            {items.map((item, index) => (
              <div key={`${item.label}-${index}`} className="flex items-center gap-2 text-small text-ink-2">
                <span
                  className={cn(
                    "size-1.5 rounded-full",
                    item.status === "error" ? "bg-against" : item.status === "running" ? "bg-amber" : "bg-for",
                  )}
                />
                <span className="min-w-0 truncate">{item.label}</span>
              </div>
            ))}
          </div>
        </div>
      );
    }
    case "aladin-actions": {
      const actions = parseActionItems(segment.body);
      if (actions.length === 0 || !onPrompt) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <div className="my-2 flex flex-wrap gap-1.5">
          {actions.map((action) => (
            <button
              key={action.prompt}
              type="button"
              onClick={() => onPrompt(action.prompt)}
              className="inline-flex min-w-0 items-center gap-1.5 rounded-chip border border-line bg-raise px-2.5 py-1.5 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
            >
              <Icon as={CornerDownRight} size="inline" mark className="shrink-0 text-amber" />
              <span className="truncate">{action.label}</span>
            </button>
          ))}
        </div>
      );
    }
    default:
      return <MarkdownText text={directiveFallback(segment)} />;
  }
}

export function parseCopilotMarkdown(text: string): MarkdownSegment[] {
  const lines = text.split(/\r?\n/);
  const segments: MarkdownSegment[] = [];
  let markdown: string[] = [];
  const flushMarkdown = () => {
    if (markdown.length === 0) return;
    segments.push({ kind: "markdown", text: markdown.join("\n") });
    markdown = [];
  };

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const leaf = /^::(aladin-[a-z0-9-]+)(\{[^}]*\})\s*$/.exec(line.trim());
    if (leaf) {
      flushMarkdown();
      segments.push({ kind: "directive", name: leaf[1], attrs: parseDirectiveAttrs(leaf[2]), body: "" });
      continue;
    }
    const container = /^::(aladin-[a-z0-9-]+)\s*$/.exec(line.trim());
    if (container) {
      const body: string[] = [];
      let closed = false;
      for (let j = i + 1; j < lines.length; j += 1) {
        if (lines[j].trim() === "::") {
          i = j;
          closed = true;
          break;
        }
        body.push(lines[j]);
      }
      if (closed) {
        flushMarkdown();
        segments.push({ kind: "directive", name: container[1], attrs: {}, body: body.join("\n") });
      } else {
        markdown.push(line, ...body);
        i = lines.length;
      }
      continue;
    }
    markdown.push(line);
  }
  flushMarkdown();
  return segments;
}

function parseDirectiveAttrs(raw: string): Record<string, string> {
  const attrs: Record<string, string> = {};
  const body = raw.slice(1, -1);
  const re = /([a-zA-Z][\w-]*)=(?:"([^"]*)"|'([^']*)'|([^\s}]+))/g;
  for (const match of body.matchAll(re)) {
    attrs[match[1]] = match[2] ?? match[3] ?? match[4] ?? "";
  }
  return attrs;
}

function directiveFallback(segment: Extract<MarkdownSegment, { kind: "directive" }>): string {
  const attrs = Object.entries(segment.attrs)
    .map(([k, v]) => `${k}="${v}"`)
    .join(" ");
  if (segment.body) return `::${segment.name}\n${segment.body}\n::`;
  return `::${segment.name}${attrs ? `{${attrs}}` : ""}`;
}

function parseActivityItems(body: string): { label: string; status: "running" | "ok" | "error" }[] {
  try {
    const parsed = JSON.parse(body);
    if (!Array.isArray(parsed)) return [];
    return parsed.flatMap((item) => {
      if (!item || typeof item !== "object") return [];
      const label = typeof item.label === "string" ? item.label.trim() : "";
      const status = item.status === "running" || item.status === "error" ? item.status : "ok";
      return label ? [{ label, status }] : [];
    });
  } catch {
    return [];
  }
}

function parseActionItems(body: string): { label: string; prompt: string }[] {
  try {
    const parsed = JSON.parse(body);
    if (!Array.isArray(parsed)) return [];
    return parsed.slice(0, 4).flatMap((item) => {
      if (!item || typeof item !== "object") return [];
      const label = typeof item.label === "string" ? item.label.trim() : "";
      const prompt = typeof item.prompt === "string" ? item.prompt.trim() : "";
      if (!label || !prompt) return [];
      return [{ label: label.slice(0, 80), prompt: prompt.slice(0, 1000) }];
    });
  } catch {
    return [];
  }
}

/** A subtle blinking caret appended to streaming text. */
export function StreamCaret({ className }: { className?: string }) {
  return <span className={cn("ml-0.5 inline-block h-3.5 w-[2px] animate-pulse bg-amber align-middle", className)} />;
}
