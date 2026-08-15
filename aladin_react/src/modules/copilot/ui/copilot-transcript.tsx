import { AlertTriangle, Check } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import type {
  CopilotCitation,
  CopilotMessageMeta,
  CopilotMessageView,
  CopilotProposal,
  CopilotToolRun,
} from "@/app/state/copilot-slice";
import { useCitationNav } from "@/modules/copilot/hooks/use-citation-nav";
import { CopilotMarkdown, StreamCaret } from "@/modules/copilot/ui/copilot-markdown";
import { cn } from "@/lib/utils";

/**
 * Everything that renders inside the transcript: turns, live tool activity, approval cards.
 *
 * These take their content as props and never reach for copilot state, so the dock stays the
 * only place that knows about the store — and each of these can be rendered in a test with a
 * literal.
 */

export function MessageBubble({
  message,
  onPrompt,
}: {
  message: CopilotMessageView;
  onPrompt: (prompt: string) => void;
}) {
  if (message.role === "user") {
    return (
      <div className="flex justify-end">
        <div className="max-w-[85%] whitespace-pre-wrap rounded-card bg-raise px-3 py-2 text-body text-ink">
          {message.content}
        </div>
      </div>
    );
  }
  return (
    <AssistantBubble
      content={message.content}
      citations={message.citations}
      meta={message.meta}
      onPrompt={onPrompt}
    />
  );
}

export function AssistantBubble({
  content,
  citations,
  meta,
  streaming,
  onPrompt,
}: {
  content: string;
  citations: CopilotCitation[];
  meta?: CopilotMessageMeta;
  streaming?: boolean;
  onPrompt: (prompt: string) => void;
}) {
  const navCitation = useCitationNav();
  // Dedupe citations by kind+id for display.
  const seen = new Set<string>();
  const unique = citations.filter((c) => {
    const key = `${c.kind}|${c.id}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
  return (
    <div className="flex flex-col gap-1.5">
      <div>
        <CopilotMarkdown text={content} onPrompt={onPrompt} />
        {streaming ? <StreamCaret /> : null}
      </div>
      {unique.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {unique.map((c) => (
            <button
              key={`${c.kind}|${c.id}`}
              type="button"
              onClick={() => navCitation(c)}
              className="max-w-[180px] truncate rounded-chip border border-line px-2 py-0.5 font-mono text-meta text-ink-3 transition-colors hover:border-amber-line hover:text-ink"
              title={`${c.kind}: ${c.title}`}
            >
              {c.title}
            </button>
          ))}
        </div>
      ) : null}
      {!streaming && turnDigest(meta) ? (
        <p className="font-mono text-meta text-ink-4">{turnDigest(meta)}</p>
      ) : null}
    </div>
  );
}

/**
 * Compact live activity: consecutive runs of the same tool collapse to one row
 * with a count; the most recent few rows stay visible while the turn runs.
 */
export function ActivityTimeline({ trail }: { trail: CopilotToolRun[] }) {
  const groups: {
    label: string;
    count: number;
    status: CopilotToolRun["status"];
    inputSummary?: string;
    resultSummary?: string;
  }[] = [];
  for (const run of trail) {
    const last = groups[groups.length - 1];
    if (last && last.label === run.label) {
      last.count += 1;
      // A group's status is its worst/latest interesting state.
      last.status = run.status === "running" ? "running" : run.status === "error" ? "error" : last.status;
      last.inputSummary = run.inputSummary ?? last.inputSummary;
      last.resultSummary = run.resultSummary ?? last.resultSummary;
    } else {
      groups.push({
        label: run.label,
        count: 1,
        status: run.status,
        inputSummary: run.inputSummary,
        resultSummary: run.resultSummary,
      });
    }
  }
  const visible = groups.slice(-6);
  return (
    <div className="rounded-card border border-line bg-field px-2.5 py-2">
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <span className="font-mono text-meta text-ink-4">activity</span>
        {groups.length > visible.length ? (
          <span className="font-mono text-meta text-ink-4">+{groups.length - visible.length}</span>
        ) : null}
      </div>
      <div className="space-y-1">
        {visible.map((g, i) => {
          const hasDetails = Boolean(g.inputSummary || g.resultSummary);
          const row = (
            <div className="flex items-center gap-2 text-small text-ink-2">
              <span
                className={cn(
                  "size-1.5 shrink-0 rounded-full",
                  g.status === "running" && "animate-pulse bg-amber",
                  g.status === "ok" && "bg-for",
                  g.status === "error" && "bg-against",
                )}
              />
              <span className="min-w-0 flex-1 truncate">{g.label}</span>
              {g.count > 1 ? <span className="font-mono text-meta text-ink-4">x{g.count}</span> : null}
            </div>
          );
          return hasDetails ? (
            <details key={`${g.label}-${i}`} className="rounded-tap open:bg-raise/40">
              <summary className="cursor-default list-none">{row}</summary>
              <div className="mt-1 space-y-0.5 border-l border-line pl-3 font-mono text-meta text-ink-4">
                {g.inputSummary ? <p className="truncate">in: {g.inputSummary}</p> : null}
                {g.resultSummary ? <p className="truncate">out: {g.resultSummary}</p> : null}
              </div>
            </details>
          ) : (
            <div key={`${g.label}-${i}`}>{row}</div>
          );
        })}
      </div>
    </div>
  );
}

export function ProposalCard({
  proposal,
  onApprove,
  onReject,
}: {
  proposal: CopilotProposal;
  onApprove: () => void;
  onReject: () => void;
}) {
  // Settled/expired proposals collapse to a one-line note.
  if (
    proposal.status === "approved" ||
    proposal.status === "rejected" ||
    proposal.status === "expired"
  ) {
    const approved = proposal.status === "approved";
    return (
      <p className="flex items-center gap-1.5 font-mono text-meta text-ink-4">
        <Icon as={Check} size="inline" mark className={cn(approved ? "text-for" : "text-ink-4")} />
        {proposal.message || (approved ? "Applied." : proposal.status === "expired" ? "That approval expired." : "Dismissed.")}
      </p>
    );
  }

  // pending: actionable. approving/rejecting: the POST is in flight — buttons lock.
  const inFlight = proposal.status !== "pending";
  return (
    <div className="rounded-card border border-amber-line bg-amber-soft/40 p-3">
      <div className="flex items-start gap-2">
        <Icon as={AlertTriangle} size="inline" mark className="mt-0.5 shrink-0 text-amber" />
        <div className="min-w-0 flex-1">
          <p className="text-small text-ink">{proposal.summary}</p>
          <p className="mt-0.5 font-mono text-meta text-ink-4">
            {proposal.status === "approving"
              ? "Approving…"
              : proposal.status === "rejecting"
                ? "Dismissing…"
                : "Needs your approval — the copilot is waiting"}
          </p>
        </div>
      </div>
      <div className="mt-2.5 flex gap-2">
        <button
          type="button"
          disabled={inFlight}
          onClick={onApprove}
          className="flex-1 rounded-chip bg-amber py-1.5 text-small font-semibold text-primary-foreground transition-opacity disabled:opacity-50"
        >
          Approve
        </button>
        <button
          type="button"
          disabled={inFlight}
          onClick={onReject}
          className="flex-1 rounded-chip border border-line py-1.5 text-small text-ink-2 transition-colors hover:text-ink disabled:opacity-50"
        >
          Reject
        </button>
      </div>
      {proposal.message ? (
        <p className="mt-1.5 font-mono text-meta text-against">{proposal.message}</p>
      ) : null}
    </div>
  );
}

/**
 * turnDigest compresses a turn's activity + cost into one muted footer line, e.g.
 * "searched ·2 · wrote shard code ·3 · built ✓ — $0.14 · 23 steps". Empty for
 * turns with no meta (legacy rows, plain Q&A with no tools and no usage).
 */
export function turnDigest(meta: CopilotMessageMeta | undefined): string {
  if (!meta) return "";
  const parts: string[] = [];
  if (meta.activity && meta.activity.length > 0) {
    const groups: { label: string; count: number; failed: boolean }[] = [];
    for (const item of meta.activity) {
      const label = toolDigestLabel(item.name);
      const last = groups[groups.length - 1];
      if (last && last.label === label) {
        last.count += 1;
        last.failed = last.failed || !item.ok;
      } else {
        groups.push({ label, count: 1, failed: !item.ok });
      }
    }
    parts.push(
      groups
        .map((g) => `${g.label}${g.count > 1 ? ` ×${g.count}` : ""}${g.failed ? " ✗" : ""}`)
        .join(" · "),
    );
  }
  const tail: string[] = [];
  if (meta.costUsd && meta.costUsd > 0) tail.push(`$${meta.costUsd.toFixed(2)}`);
  if (meta.numTurns && meta.numTurns > 1) tail.push(`${meta.numTurns} steps`);
  if (tail.length > 0) parts.push(tail.join(" · "));
  return parts.join(" — ");
}

function toolDigestLabel(name: string): string {
  switch (name) {
    case "search":
      return "searched workspace";
    case "get_entity":
      return "read entity";
    case "get_insights":
      return "read insights";
    case "list_artifacts":
      return "listed artifacts";
    case "get_artifact":
      return "read artifact";
    case "get_page":
    case "list_pages":
      return "read page";
    case "get_watchlist":
      return "checked watchlist";
    case "get_browser_tree":
    case "list_folders":
      return "browsed workspace";
    case "search_pages":
      return "searched pages";
    case "get_bars":
      return "read price history";
    case "get_quote":
      return "fetched quote";
    case "get_news":
      return "read news";
    case "get_movers":
      return "scanned movers";
    case "get_most_actives":
      return "checked most-actives";
    case "get_account":
      return "read account";
    case "get_positions":
      return "read positions";
    case "create_alert":
      return "set alert";
    case "list_alerts":
      return "checked alerts";
    case "delete_alert":
      return "removed alert";
    case "create_app":
      return "created shard";
    case "list_dir":
    case "read_file":
      return "read shard files";
    case "write_file":
    case "edit_file":
      return "wrote shard code";
    case "install_lib":
      return "added dependency";
    case "build_app":
      return "built shard";
    case "delete_file":
      return "deleted shard file";
    case "publish_app":
      return "published shard";
    case "preview_open":
    case "preview_navigate":
    case "preview_snapshot":
    case "preview_screenshot":
    case "preview_eval":
    case "preview_click":
    case "preview_console":
    case "preview_close":
    case "preview_restart":
      return "previewed shard";
    case "create_page":
      return "created page";
    case "insert_blocks":
    case "update_block":
    case "update_page":
      return "edited page";
    case "delete_block":
      return "removed block";
    case "add_to_watchlist":
      return "updated watchlist";
    case "list_watchlists":
      return "listed watchlists";
    case "create_watchlist":
      return "created watchlist";
    case "draw_edge":
      return "linked entities";
    default:
      return name.replaceAll("_", " ");
  }
}
