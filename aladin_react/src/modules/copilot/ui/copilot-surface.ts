import { BarChart3, FileText, Focus, UserRound, type LucideIcon } from "lucide-react";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import { ARTIFACT_ICONS } from "@/modules/workspace/ui/kind-icons";

/**
 * Surface → copy. What the copilot says about what the user is looking at.
 *
 * Deliberately free of React: these are total functions from a `CopilotSurface` to strings and
 * plain data, which is why they can be tested by calling them rather than by rendering a dock.
 * The icons here are component references in a data structure, not JSX.
 */

export interface ScopeSummary {
  title: string;
  kind: string;
  icon: LucideIcon;
  rows: { label: string; value: string }[];
}

export function suggestionsFor(surface: CopilotSurface): string[] {
  switch (surface.kind) {
    case "ticker": {
      const s = surface.symbol ?? "this ticker";
      return [`What's my thesis on ${s}?`, `How does ${s} look technically?`, `Any recent notes on ${s}?`];
    }
    case "entity":
      return ["What do I know about this?", "What's it connected to?"];
    case "artifact":
    case "page":
    case "shard": {
      if (surface.artifactKind === "app") {
        return [
          "Summarize this shard",
          "What would make this clearer?",
          "Polish the interaction design",
        ];
      }
      if (surface.artifactKind === "file") {
        return [
          "Summarize this source",
          "What are the key claims here?",
          "Extract the useful citations",
        ];
      }
      return [
        "Summarize what I'm looking at",
        "What are the key claims here?",
        "Turn this into an interactive shard",
      ];
    }
    case "markets":
      return ["What am I watching?", "Anything notable in my watchlist?"];
    default:
      return [
        "What have I been researching?",
        "Summarize my recent insights",
        "Build a shard about a ticker I follow",
      ];
  }
}

/** Placeholder teaches the current mode: normal ask, surface-scoped ask, or queueing. */
export function composerPlaceholder(busy: boolean, surfaceLabel: string | null): string {
  if (busy) return "Type a follow-up — sends when this turn finishes…";
  if (surfaceLabel) return `Ask about ${surfaceLabel}…`;
  return "Ask the copilot…";
}

export function describeSurface(surface: CopilotSurface): string | null {
  if (surface.kind === "ticker" && surface.symbol) return surface.symbol;
  if (surface.kind === "entity") return surface.label ?? "this entity";
  if (surface.kind === "artifact" || surface.kind === "page" || surface.kind === "shard") {
    if (surface.label) return surface.label;
    return `this ${surfaceKindLabel(surface.artifactKind)}`;
  }
  if (surface.kind === "markets") return "Markets";
  return null;
}

export function scopeForSurface(surface: CopilotSurface, label: string | null): ScopeSummary | null {
  if (!label) return null;
  switch (surface.kind) {
    case "ticker": {
      const symbol = surface.symbol?.toUpperCase() ?? label;
      return {
        title: symbol,
        kind: "ticker",
        icon: BarChart3,
        rows: [{ label: "symbol", value: symbol }],
      };
    }
    case "entity":
      return {
        title: label,
        kind: "entity",
        icon: UserRound,
        rows: surface.id ? [{ label: "id", value: surface.id }] : [],
      };
    case "artifact":
    case "page":
    case "shard": {
      const kind = surfaceKindLabel(surface.artifactKind);
      return {
        title: label,
        kind,
        icon: surface.artifactKind ? ARTIFACT_ICONS[surface.artifactKind] : FileText,
        rows: [
          ...(surface.id ? [{ label: "id", value: surface.id }] : []),
          ...(surface.artifactKind ? [{ label: "type", value: surface.artifactKind }] : []),
        ],
      };
    }
    case "markets":
      return {
        title: "Markets",
        kind: "market surface",
        icon: BarChart3,
        rows: [{ label: "scope", value: "watchlists" }],
      };
    default:
      return {
        title: label,
        kind: surface.kind || "surface",
        icon: Focus,
        rows: [],
      };
  }
}

export function surfaceKindLabel(kind: CopilotSurface["artifactKind"]): string {
  switch (kind) {
    case "app":
      return "shard";
    case "file":
      return "source";
    case "link":
      return "link";
    case "voice":
      return "voice note";
    case "note":
      return "page";
    default:
      return "item";
  }
}
