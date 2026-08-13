import { useLocation } from "react-router-dom";
import { useMemo } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { CopilotSurface } from "@/repos/copilot/copilot-repo";
import { activeArtifactIdOf } from "@/modules/workspace/domain";
import { useObservableState } from "@/shared/flow/use-observable-state";
import type { ArtifactPreview, BrowserTreeNode } from "@/shared/api/models";

/**
 * Derives what the user is currently looking at, so every copilot turn is context-aware.
 * Precedence: an open ticker modal wins (it's a focused overlay), then the route, then the
 * open page. Read reactively so the surface follows navigation.
 */
export function useCurrentSurface(): CopilotSurface {
  const location = useLocation();
  const { services } = useAppComposition();
  const openTickerSymbol = useAppStore((s) => s.openTickerSymbol);
  const activeArtifactId = useAppStore((s) => activeArtifactIdOf(s.workspace));
  const treeStream = useMemo(() => services.workspace.tree(), [services.workspace]);
  const treeLoadable = useObservableState(treeStream);
  const tree = treeLoadable.status === "data" ? treeLoadable.value : [];
  const activeArtifactPreview = activeArtifactId
    ? findArtifactPreview(tree, activeArtifactId)
    : null;

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
  // subject. Keep the wire kind as "artifact" so the backend still preloads the editable
  // context block; attach the visible title for composer copy and prompt hints.
  if ((path.startsWith("/folders") || path.startsWith("/home")) && activeArtifactId) {
    return {
      kind: "artifact",
      id: activeArtifactId,
      label: activeArtifactPreview?.title,
      artifactKind: activeArtifactPreview?.kind,
    };
  }

  // Fall back to the bare route name (insights, entities, sources, graph, …).
  const seg = path.split("/").filter(Boolean)[0];
  return { kind: seg || "home" };
}

function findArtifactPreview(
  nodes: BrowserTreeNode[],
  artifactId: string,
): ArtifactPreview | null {
  for (const node of nodes) {
    if (node.artifactPreview?.id === artifactId) return node.artifactPreview;
    const child = findArtifactPreview(node.children, artifactId);
    if (child) return child;
  }
  return null;
}
