import { useLocation } from "react-router-dom";
import { useAppStore } from "@/app/state/store";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import { activeArtifactIdOf } from "@/modules/workspace/domain";

/**
 * Derives what the user is currently looking at, so every copilot turn is context-aware.
 * Precedence: an open ticker modal wins (it's a focused overlay), then the route, then the
 * open page. Read reactively so the surface follows navigation.
 */
export function useCurrentSurface(): CopilotSurface {
  const location = useLocation();
  const openTickerSymbol = useAppStore((s) => s.openTickerSymbol);
  const activeArtifactId = useAppStore((s) => activeArtifactIdOf(s.workspace));

  // The global ticker modal is a focused overlay — if it's open, that's the subject.
  if (openTickerSymbol) {
    return { kind: "ticker", symbol: openTickerSymbol, label: openTickerSymbol };
  }

  const path = location.pathname;
  if (path.startsWith("/entity/")) {
    const id = path.slice("/entity/".length);
    if (id) return { kind: "entity", id };
  }
  if (path.startsWith("/markets")) {
    return { kind: "markets" };
  }

  // On a folders/home surface, an open artifact (page OR shard OR link/file) is the likely
  // subject. We can't cheaply tell the kind here (it lives in an async cache), so pass a generic
  // "artifact" surface + the id — the agent reads it via get_artifact and learns the kind.
  if ((path.startsWith("/folders") || path.startsWith("/home")) && activeArtifactId) {
    return { kind: "artifact", id: activeArtifactId };
  }

  // Fall back to the bare route name (insights, entities, sources, graph, …).
  const seg = path.split("/").filter(Boolean)[0];
  return { kind: seg || "home" };
}
