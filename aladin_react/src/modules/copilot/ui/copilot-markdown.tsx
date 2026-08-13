import {
  Activity,
  AlertCircle,
  AlertTriangle,
  AppWindow,
  BarChart3,
  CheckCircle2,
  CornerDownRight,
  ExternalLink,
  FileText,
  GitCompare,
  Layers,
  RefreshCw,
  Send,
  UserRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
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
    case "aladin-entity": {
      const id = segment.attrs.id;
      const title = segment.attrs.title || id;
      const kind = segment.attrs.kind || "entity";
      if (!id || !title) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <button
          type="button"
          onClick={() => onNavigate("entity", id, title)}
          className="my-2 flex w-full items-center gap-2 rounded-card border border-line bg-field px-2.5 py-2 text-left transition-colors hover:border-amber-line"
        >
          <Icon as={UserRound} mark className="shrink-0 text-amber" />
          <span className="min-w-0">
            <span className="block truncate text-small font-semibold text-ink">{title}</span>
            <span className="block font-mono text-meta text-ink-4">{kind}</span>
          </span>
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
            {items.map((item, index) => {
              const details = activityDetails(item);
              const row = (
                <div className="flex items-center gap-2 text-small text-ink-2">
                  <span
                    className={cn(
                      "size-1.5 shrink-0 rounded-full",
                      item.status === "error" ? "bg-against" : item.status === "running" ? "bg-amber" : "bg-for",
                    )}
                  />
                  <span className="min-w-0 flex-1 truncate">{item.label}</span>
                  {item.time ? <span className="shrink-0 font-mono text-meta text-ink-4">{item.time}</span> : null}
                </div>
              );
              return details.length > 0 ? (
                <details key={`${item.label}-${index}`} className="rounded-tap open:bg-raise/50">
                  <summary className="cursor-default list-none">{row}</summary>
                  <div className="mt-1 space-y-0.5 border-l border-line pl-3 font-mono text-meta text-ink-4">
                    {details.map((detail) => (
                      <p key={detail} className="truncate">
                        {detail}
                      </p>
                    ))}
                  </div>
                </details>
              ) : (
                <div key={`${item.label}-${index}`}>{row}</div>
              );
            })}
          </div>
        </div>
      );
    }
    case "aladin-actions": {
      const actions = parseActionItems(segment.body);
      if (actions.length === 0) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <div className="my-2 flex flex-wrap gap-1.5">
          {actions.map((action) => (
            <button
              key={`${action.action}-${action.target}`}
              type="button"
              onClick={() => {
                if ((action.action === "send_prompt" || action.action === "continue" || action.action === "retry") && onPrompt) {
                  onPrompt(action.prompt);
                }
                if (action.action === "open_artifact") onNavigate(action.kind, action.id, action.label);
                if (action.action === "open_ticker") onNavigate("ticker", action.symbol, action.symbol);
              }}
              disabled={isPromptAction(action) && !onPrompt}
              className="inline-flex min-w-0 items-center gap-1.5 rounded-chip border border-line bg-raise px-2.5 py-1.5 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
            >
              <Icon as={actionIcon(action)} size="inline" mark className="shrink-0 text-amber" />
              <span className="truncate">{action.label}</span>
            </button>
          ))}
        </div>
      );
    }
    case "aladin-approval": {
      const approval = parseApprovalBlock(segment.attrs, segment.body);
      if (!approval) return <MarkdownText text={directiveFallback(segment)} />;
      const pending = approval.status === "pending";
      return (
        <div
          className={cn(
            "my-2 rounded-card border p-3",
            pending ? "border-amber-line bg-amber-soft/40" : "border-line bg-field",
          )}
        >
          <div className="flex items-start gap-2">
            <Icon
              as={pending ? AlertTriangle : CheckCircle2}
              size="inline"
              mark
              className={cn("mt-0.5 shrink-0", pending ? "text-amber" : "text-for")}
            />
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-2">
                <p className="min-w-0 flex-1 truncate text-small font-semibold text-ink">{approval.action}</p>
                <span className="shrink-0 rounded-chip border border-line bg-raise px-1.5 py-0.5 font-mono text-meta text-ink-4">
                  {approval.status}
                </span>
              </div>
              <p className="mt-0.5 truncate text-small text-ink-2">{approval.target}</p>
              {approval.risk ? <p className="mt-1 text-small text-ink-3">{approval.risk}</p> : null}
            </div>
          </div>
          {approval.details.length > 0 ? (
            <div className="mt-2 space-y-0.5 border-l border-line pl-3 font-mono text-meta text-ink-4">
              {approval.details.map((detail) => (
                <p key={detail} className="truncate">
                  {detail}
                </p>
              ))}
            </div>
          ) : null}
        </div>
      );
    }
    case "aladin-diff": {
      const diff = parseDiffBlock(segment.attrs, segment.body);
      if (!diff) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <div className="my-2 rounded-card border border-line bg-field">
          <div className="flex items-center gap-2 border-b border-line px-2.5 py-2">
            <Icon as={GitCompare} size="inline" mark className="shrink-0 text-amber" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-small font-semibold text-ink">{diff.title}</p>
              {diff.path ? <p className="truncate font-mono text-meta text-ink-4">{diff.path}</p> : null}
            </div>
            <span className="shrink-0 font-mono text-meta text-ink-4">
              +{diff.added} / -{diff.removed}
            </span>
          </div>
          <div className="max-h-72 overflow-auto p-2 font-mono text-meta">
            {diff.lines.map((line, index) => (
              <pre
                key={`${line.kind}-${index}-${line.text}`}
                className={cn(
                  "min-w-0 overflow-hidden text-ellipsis whitespace-pre-wrap rounded-tap px-1.5 py-0.5",
                  line.kind === "add" && "bg-for-soft text-for",
                  line.kind === "remove" && "bg-against-soft text-against",
                  line.kind === "context" && "text-ink-4",
                )}
              >
                {line.kind === "add" ? "+" : line.kind === "remove" ? "-" : " "}
                {line.text}
              </pre>
            ))}
          </div>
        </div>
      );
    }
    case "aladin-shard-preview": {
      const preview = parseShardPreviewBlock(segment.attrs, segment.body);
      if (!preview) return <MarkdownText text={directiveFallback(segment)} />;
      const errored = preview.status === "error";
      const previewId = preview.id;
      return (
        <div className="my-2 rounded-card border border-line bg-field">
          <div className="flex items-start gap-2 border-b border-line px-2.5 py-2">
            <Icon as={AppWindow} size="inline" mark className="mt-0.5 shrink-0 text-amber" />
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-2">
                <p className="min-w-0 flex-1 truncate text-small font-semibold text-ink">{preview.title}</p>
                <span
                  className={cn(
                    "shrink-0 rounded-chip border px-1.5 py-0.5 font-mono text-meta",
                    errored ? "border-against/30 bg-against-soft text-against" : "border-line bg-raise text-ink-4",
                  )}
                >
                  {preview.status}
                </span>
              </div>
              <p className="mt-0.5 truncate font-mono text-meta text-ink-4">{preview.subtitle}</p>
            </div>
          </div>
          {preview.diagnostics.length > 0 ? (
            <div className="space-y-0.5 px-2.5 py-2 font-mono text-meta text-ink-4">
              {preview.diagnostics.map((diagnostic) => (
                <p key={diagnostic} className={cn("truncate", errored && "text-against")}>
                  {diagnostic}
                </p>
              ))}
            </div>
          ) : null}
          {previewId ? (
            <div className="border-t border-line px-2.5 py-2">
              <button
                type="button"
                onClick={() => onNavigate("shard", previewId, preview.title)}
                className="inline-flex max-w-full items-center gap-1.5 rounded-chip border border-line bg-raise px-2.5 py-1.5 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
              >
                <Icon as={ExternalLink} size="inline" mark className="shrink-0 text-amber" />
                <span className="truncate">Open shard</span>
              </button>
            </div>
          ) : null}
        </div>
      );
    }
    case "aladin-error-recovery": {
      const recovery = parseErrorRecoveryBlock(segment.body);
      if (!recovery) return <MarkdownText text={directiveFallback(segment)} />;
      return (
        <div className="my-2 rounded-card border border-against/30 bg-against-soft/40 p-3">
          <div className="flex items-start gap-2">
            <Icon as={AlertCircle} size="inline" mark className="mt-0.5 shrink-0 text-against" />
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-2">
                <p className="min-w-0 flex-1 truncate text-small font-semibold text-ink">{recovery.title}</p>
                {recovery.code ? (
                  <span className="shrink-0 rounded-chip border border-line bg-raise px-1.5 py-0.5 font-mono text-meta text-ink-4">
                    {recovery.code}
                  </span>
                ) : null}
              </div>
              <p className="mt-1 text-small text-ink-2">{recovery.message}</p>
            </div>
          </div>
          {recovery.actions.length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {recovery.actions.map((action) => (
                <button
                  key={`${action.action}-${action.target}`}
                  type="button"
                  onClick={() => {
                    if (isPromptAction(action) && onPrompt) onPrompt(action.prompt);
                    if (action.action === "open_artifact") onNavigate(action.kind, action.id, action.label);
                    if (action.action === "open_ticker") onNavigate("ticker", action.symbol, action.symbol);
                  }}
                  disabled={isPromptAction(action) && !onPrompt}
                  className="inline-flex min-w-0 items-center gap-1.5 rounded-chip border border-line bg-field px-2.5 py-1.5 text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
                >
                  <Icon as={actionIcon(action)} size="inline" mark className="shrink-0 text-amber" />
                  <span className="truncate">{action.label}</span>
                </button>
              ))}
            </div>
          ) : null}
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

export type ActivityItem = {
  label: string;
  status: "running" | "ok" | "error";
  detail?: string;
  inputSummary?: string;
  resultSummary?: string;
  time?: string;
};

export function parseActivityItems(body: string): ActivityItem[] {
  try {
    const parsed = JSON.parse(body);
    const rawItems: unknown[] = Array.isArray(parsed)
      ? parsed
      : parsed && typeof parsed === "object" && Array.isArray(parsed.items)
        ? (parsed.items as unknown[])
        : [];
    return rawItems.slice(0, 12).flatMap((item) => {
      if (!item || typeof item !== "object") return [];
      const record = item as Record<string, unknown>;
      const label = textField(record.label, 120);
      const status = record.status === "running" || record.status === "error" ? record.status : "ok";
      const detail = textField(record.detail ?? record.message ?? record.error, 500);
      const inputSummary = textField(record.inputSummary, 500);
      const resultSummary = textField(record.resultSummary, 500);
      const time = textField(record.time ?? record.finishedAt ?? record.startedAt, 80);
      return label ? [{ label, status, detail, inputSummary, resultSummary, time }] : [];
    });
  } catch {
    return [];
  }
}

function activityDetails(item: ActivityItem): string[] {
  return [
    item.inputSummary ? `in: ${item.inputSummary}` : "",
    item.resultSummary ? `out: ${item.resultSummary}` : "",
    item.detail ?? "",
  ].filter(Boolean);
}

function textField(value: unknown, max: number): string | undefined {
  if (typeof value !== "string") return undefined;
  const text = value.replace(/\s+/g, " ").trim();
  if (!text) return undefined;
  return text.length > max ? `${text.slice(0, max)}…` : text;
}

export type ActionItem =
  | { action: "send_prompt" | "continue" | "retry"; label: string; prompt: string; target: string }
  | { action: "open_artifact"; label: string; id: string; kind: string; target: string }
  | { action: "open_ticker"; label: string; symbol: string; target: string };

export function parseActionItems(body: string): ActionItem[] {
  try {
    const parsed = JSON.parse(body);
    if (!Array.isArray(parsed)) return [];
    const actions: ActionItem[] = [];
    for (const item of parsed.slice(0, 4)) {
      if (!item || typeof item !== "object") continue;
      const label = typeof item.label === "string" ? item.label.trim() : "";
      const action = typeof item.action === "string" ? item.action : "";
      if (!label) continue;
      if (action === "send_prompt" || action === "continue" || action === "retry") {
        const fallbackPrompt = action === "continue" ? "continue" : action === "retry" ? "try again" : "";
        const prompt = typeof item.prompt === "string" ? item.prompt.trim() : fallbackPrompt;
        if (prompt) {
          actions.push({ action, label: label.slice(0, 80), prompt: prompt.slice(0, 1000), target: prompt });
        }
        continue;
      }
      if (action === "open_artifact") {
        const id = typeof item.artifactId === "string" ? item.artifactId.trim() : typeof item.id === "string" ? item.id.trim() : "";
        const rawKind = typeof item.kind === "string" ? item.kind.trim() : "page";
        const kind = safeArtifactKind(rawKind);
        if (id) {
          actions.push({ action, label: label.slice(0, 80), id: id.slice(0, 160), kind, target: id });
        }
        continue;
      }
      if (action === "open_ticker") {
        const symbol = typeof item.symbol === "string" ? item.symbol.trim().toUpperCase() : "";
        if (isSafeSymbol(symbol)) actions.push({ action, label: label.slice(0, 80), symbol, target: symbol });
        continue;
      }
      const prompt = typeof item.prompt === "string" ? item.prompt.trim() : "";
      if (prompt) {
        actions.push({
          action: "send_prompt",
          label: label.slice(0, 80),
          prompt: prompt.slice(0, 1000),
          target: prompt,
        });
      }
    }
    return actions;
  } catch {
    return [];
  }
}

function isPromptAction(action: ActionItem): action is Extract<ActionItem, { prompt: string }> {
  return action.action === "send_prompt" || action.action === "continue" || action.action === "retry";
}

function actionIcon(action: ActionItem): LucideIcon {
  if (action.action === "open_artifact") return FileText;
  if (action.action === "open_ticker") return BarChart3;
  if (action.action === "retry") return RefreshCw;
  if (action.action === "continue") return CornerDownRight;
  return Send;
}

function safeArtifactKind(kind: string): string {
  if (kind === "app") return "shard";
  return ["page", "shard", "document", "artifact"].includes(kind) ? kind : "page";
}

function isSafeSymbol(symbol: string): boolean {
  return /^[A-Z][A-Z0-9.-]{0,11}$/.test(symbol);
}

export type ApprovalBlock = {
  action: string;
  target: string;
  status: "pending" | "approved" | "rejected" | "expired";
  risk?: string;
  details: string[];
};

export function parseApprovalBlock(attrs: Record<string, string>, body: string): ApprovalBlock | null {
  let data: Record<string, unknown> = {};
  if (body.trim()) {
    try {
      const parsed = JSON.parse(body);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
      data = parsed as Record<string, unknown>;
    } catch {
      return null;
    }
  }

  const merged: Record<string, unknown> = { ...data, ...attrs };
  const action = textField(merged.action ?? merged.summary ?? merged.tool, 120);
  const target = textField(merged.target ?? merged.title ?? merged.path ?? merged.artifactId ?? merged.id, 160);
  if (!action || !target) return null;
  const status = approvalStatus(merged.status);
  const risk = textField(merged.risk ?? merged.message, 280);
  const details = parseApprovalDetails(merged.details ?? merged.detail ?? merged.exactAction);
  return { action, target, status, risk, details };
}

function approvalStatus(value: unknown): ApprovalBlock["status"] {
  return value === "approved" || value === "rejected" || value === "expired" ? value : "pending";
}

function parseApprovalDetails(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap((v) => textField(v, 220) ?? []).slice(0, 6);
  const detail = textField(value, 220);
  return detail ? [detail] : [];
}

export type DiffBlock = {
  title: string;
  path?: string;
  added: number;
  removed: number;
  lines: { kind: "add" | "remove" | "context"; text: string }[];
};

export function parseDiffBlock(attrs: Record<string, string>, body: string): DiffBlock | null {
  let data: Record<string, unknown> = {};
  if (body.trim().startsWith("{")) {
    try {
      const parsed = JSON.parse(body);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
      data = parsed as Record<string, unknown>;
    } catch {
      return null;
    }
  }

  const merged: Record<string, unknown> = { ...data, ...attrs };
  const title = textField(merged.title ?? merged.summary ?? merged.path, 120) ?? "Changes";
  const path = textField(merged.path ?? merged.file, 180);
  const rawLines = Array.isArray(merged.lines) ? merged.lines : parseUnifiedDiffLines(body);
  const lines = rawLines.slice(0, 80).flatMap((line) => parseDiffLine(line));
  if (lines.length === 0) return null;
  const added = lines.filter((line) => line.kind === "add").length;
  const removed = lines.filter((line) => line.kind === "remove").length;
  return { title, path, added, removed, lines };
}

function parseUnifiedDiffLines(body: string): unknown[] {
  return body
    .split(/\r?\n/)
    .filter((line) => line.startsWith("+") || line.startsWith("-") || line.startsWith(" "))
    .filter((line) => !line.startsWith("+++") && !line.startsWith("---"));
}

function parseDiffLine(value: unknown): DiffBlock["lines"] {
  if (typeof value === "string") {
    const prefix = value[0];
    const text = textField(value.slice(prefix === "+" || prefix === "-" || prefix === " " ? 1 : 0), 220);
    if (!text) return [];
    if (prefix === "+") return [{ kind: "add", text }];
    if (prefix === "-") return [{ kind: "remove", text }];
    return [{ kind: "context", text }];
  }
  if (!value || typeof value !== "object") return [];
  const record = value as Record<string, unknown>;
  const text = textField(record.text ?? record.value, 220);
  if (!text) return [];
  const kind = record.kind === "add" || record.kind === "remove" ? record.kind : "context";
  return [{ kind, text }];
}

export type ShardPreviewBlock = {
  id?: string;
  title: string;
  status: "building" | "ready" | "published" | "error";
  subtitle: string;
  diagnostics: string[];
};

export function parseShardPreviewBlock(attrs: Record<string, string>, body: string): ShardPreviewBlock | null {
  let data: Record<string, unknown> = {};
  if (body.trim()) {
    try {
      const parsed = JSON.parse(body);
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
      data = parsed as Record<string, unknown>;
    } catch {
      return null;
    }
  }
  const merged: Record<string, unknown> = { ...data, ...attrs };
  const id = textField(merged.artifactId ?? merged.pageId ?? merged.id, 160);
  const title = textField(merged.title ?? merged.name ?? id, 120);
  if (!title) return null;
  const status = shardPreviewStatus(merged.status, merged.buildOk ?? merged.ok);
  const url = safePreviewPath(merged.previewUrl ?? merged.url);
  const subtitle =
    textField(merged.subtitle ?? merged.summary, 160) ??
    (url ? `preview: ${url}` : status === "error" ? "build needs attention" : "preview ready");
  const diagnostics = parsePreviewDiagnostics(merged.diagnostics ?? merged.errors ?? merged.log);
  return { id, title, status, subtitle, diagnostics };
}

function shardPreviewStatus(status: unknown, ok: unknown): ShardPreviewBlock["status"] {
  if (status === "building" || status === "ready" || status === "published" || status === "error") return status;
  if (ok === false) return "error";
  if (ok === true) return "ready";
  return "ready";
}

function safePreviewPath(value: unknown): string | undefined {
  const path = textField(value, 180);
  if (!path || !path.startsWith("/") || path.startsWith("//")) return undefined;
  return path;
}

function parsePreviewDiagnostics(value: unknown): string[] {
  if (Array.isArray(value)) return value.flatMap((v) => textField(v, 220) ?? []).slice(0, 8);
  if (typeof value === "string") {
    return value
      .split(/\r?\n/)
      .flatMap((line) => textField(line, 220) ?? [])
      .slice(0, 8);
  }
  return [];
}

export type ErrorRecoveryBlock = {
  title: string;
  message: string;
  code?: string;
  actions: ActionItem[];
};

export function parseErrorRecoveryBlock(body: string): ErrorRecoveryBlock | null {
  if (!body.trim()) return null;
  let data: Record<string, unknown>;
  try {
    const parsed = JSON.parse(body);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return null;
    data = parsed as Record<string, unknown>;
  } catch {
    return null;
  }
  const message = textField(data.message ?? data.error ?? data.detail, 500);
  if (!message) return null;
  const title = textField(data.title, 120) ?? "Couldn’t complete that";
  const code = textField(data.code, 80);
  const actions = parseRecoveryActions(data);
  return { title, message, code, actions };
}

function parseRecoveryActions(data: Record<string, unknown>): ActionItem[] {
  if (Array.isArray(data.actions)) return parseActionItems(JSON.stringify(data.actions));
  const retryPrompt = textField(data.retryPrompt, 1000);
  if (!retryPrompt) return [];
  return [{ action: "retry", label: "Try again", prompt: retryPrompt, target: retryPrompt }];
}

/** A subtle blinking caret appended to streaming text. */
export function StreamCaret({ className }: { className?: string }) {
  return <span className={cn("ml-0.5 inline-block h-3.5 w-[2px] animate-pulse bg-amber align-middle", className)} />;
}
