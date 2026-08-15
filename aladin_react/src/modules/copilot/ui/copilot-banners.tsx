import { Sparkles, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import { suggestionsFor } from "@/modules/copilot/ui/copilot-surface";
import { cn } from "@/lib/utils";

/**
 * The dock's transient notices — the things that appear above or instead of the transcript.
 *
 * Every one of these returns null for its "nothing to say" case, so the dock can mount them
 * unconditionally and stay a flat list of slots rather than a stack of ternaries.
 */

export function RealtimeStatusBanner({ state }: { state: "connecting" | "open" | "closed" }) {
  if (state === "open") return null;
  return (
    <div className="mb-1.5 flex items-center gap-2 rounded-card border border-line bg-raise px-2.5 py-1.5">
      <span
        aria-hidden
        className={cn(
          "size-1.5 rounded-full",
          state === "connecting" ? "animate-pulse bg-amber" : "bg-against",
        )}
      />
      <p className="font-mono text-meta text-ink-3">
        {state === "connecting" ? "reconnecting stream…" : "stream offline — reconnecting"}
      </p>
    </div>
  );
}

export function LatestTranscriptButton({
  visible,
  onClick,
}: {
  visible: boolean;
  onClick: () => void;
}) {
  if (!visible) return null;
  return (
    <div className="pointer-events-none relative">
      <button
        type="button"
        onClick={onClick}
        className="pointer-events-auto absolute bottom-2 left-1/2 -translate-x-1/2 rounded-chip border border-line bg-raise px-2.5 py-1 font-mono text-meta text-ink-2 shadow-panel transition-colors hover:border-amber-line hover:text-ink"
      >
        ↓ latest
      </button>
    </div>
  );
}

export function EmptyTranscriptState({
  surface,
  surfaceLabel,
  onPrompt,
}: {
  surface: CopilotSurface;
  surfaceLabel: string | null;
  onPrompt: (prompt: string) => void;
}) {
  return (
    <div className="mt-6 flex flex-col items-center gap-3 px-4 text-center">
      {/* Empty-state illustration, not chrome — deliberately off the <Icon>
          scale, so §5 rule 9's grep flags it on purpose. */}
      <Sparkles className="size-5 text-ink-4" strokeWidth={1.5} />
      <p className="text-body text-ink-3">Ask about your research — grounded in your Aladin data.</p>
      {surfaceLabel ? (
        <p className="font-mono text-meta text-ink-4">Looking at {surfaceLabel}</p>
      ) : null}
      <div className="mt-1 flex flex-col items-stretch gap-1.5">
        {suggestionsFor(surface).map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => onPrompt(s)}
            className="rounded-chip border border-line px-3 py-1.5 text-left text-small text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
          >
            {s}
          </button>
        ))}
      </div>
    </div>
  );
}

export function CopilotErrorBanner({
  error,
  code,
  onContinue,
}: {
  error: string | null;
  code: string | null;
  onContinue: () => void;
}) {
  if (!error) return null;
  return (
    <div className="rounded-card border border-against/40 bg-against/10 px-3 py-2">
      <p className="text-small text-against">{error}</p>
      {code === "max_turns" ? (
        <button
          type="button"
          onClick={onContinue}
          className="mt-1.5 rounded-chip border border-line px-2.5 py-1 text-meta text-ink-2 transition-colors hover:border-amber-line hover:text-ink"
        >
          Continue where it left off
        </button>
      ) : null}
    </div>
  );
}

export function HealthWarningBanner({
  message,
  onDismiss,
}: {
  message: string | null;
  onDismiss: () => void;
}) {
  if (!message) return null;
  return (
    <div className="mb-1.5 flex items-start justify-between gap-2 rounded-card border border-amber-line bg-amber-soft/40 px-2.5 py-1.5">
      <p className="text-meta text-ink-2">{message}</p>
      <button
        type="button"
        onClick={onDismiss}
        aria-label="Dismiss warning"
        className="text-ink-4 hover:text-ink"
      >
        <Icon as={X} size="inline" mark />
      </button>
    </div>
  );
}

export function QueuedFollowupBanner({
  text,
  onClear,
}: {
  text: string | null;
  onClear: () => void;
}) {
  if (!text) return null;
  return (
    <div className="mb-1.5 flex items-center justify-between gap-2 rounded-card border border-line bg-raise px-2.5 py-1.5">
      <p className="truncate font-mono text-meta text-ink-3">
        queued — sends when the copilot finishes: “{text}”
      </p>
      <button
        type="button"
        onClick={onClear}
        aria-label="Remove queued message"
        className="text-ink-4 hover:text-ink"
      >
        <Icon as={X} size="inline" mark />
      </button>
    </div>
  );
}
