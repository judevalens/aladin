import { useCallback, useEffect, useMemo, useState } from "react";
import { debounceTime, filter } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import { useObservableState } from "@/shared/flow/use-observable-state";
import { findFolderChildren, isContainerKind } from "@/modules/workspace/domain";
import type { ResearchFolder } from "@/repos/research/research-repo";
import type { BrowserTreeNode } from "@/shared/api/models";

/** One piece of captured material living in the research folder (§21). */
export interface ResearchMaterial {
  id: string;
  title: string;
  kind: string;
  isContainer: boolean;
  updatedLabel?: string | null;
}

export interface UseResearchOverview {
  folder: ResearchFolder | null;
  /** What's actually in the folder — the material this strategy is being built from. */
  material: ResearchMaterial[];
  loading: boolean;
  error: string | null;
  saving: boolean;
  onSaveHypothesis: (hypothesis: string) => Promise<void>;
}

/**
 * Loads a research folder's strategy facts for the Overview (RESEARCH_SURFACE_PRD §15).
 *
 * These are the fields the tree frame deliberately doesn't carry, so they're fetched when
 * the Overview opens. Staying fresh still rides the syncer rather than an invalidation
 * signal: every research write emits the node's upsert frame server-side, so we refetch
 * off that event — the same shape use-graph-pane uses for a derived aggregate.
 */
export function useResearchOverview(nodeId: string): UseResearchOverview {
  const { repos, runtime, services } = useAppComposition();
  const [folder, setFolder] = useState<ResearchFolder | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  // Reactive via the syncer: a node frame for THIS research folder means something about
  // it changed (rename, hypothesis, later run state), so refetch the heavy fields.
  useEffect(() => {
    if (!nodeId) return;
    const sub = runtime.dataEvents
      .events()
      .pipe(
        filter((event) => event.type === "nodeUpserted" && event.payload.id === nodeId),
        debounceTime(250),
      )
      .subscribe(() => reload());
    return () => sub.unsubscribe();
  }, [runtime, nodeId, reload]);

  useEffect(() => {
    if (!nodeId) {
      setFolder(null);
      setError(null);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.research
      .get(nodeId)
      .then((result) => {
        if (cancelled) return;
        setFolder(result);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (cancelled) return;
        setError(e instanceof Error ? e.message : String(e));
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos, nodeId, nonce]);

  const onSaveHypothesis = useCallback(
    async (hypothesis: string) => {
      setSaving(true);
      try {
        await repos.research.setHypothesis(nodeId, hypothesis);
        // The write emits a node frame; the subscription above brings the fresh copy
        // back. Nothing is patched optimistically.
      } finally {
        setSaving(false);
      }
    },
    [repos, nodeId],
  );

  // The folder's contents come from the tree the syncer already maintains — no second
  // fetch, and it stays live because the tree does.
  const treeLoadable = useObservableState(services.workspace.tree());
  const tree: BrowserTreeNode[] = treeLoadable.status === "data" ? treeLoadable.value : [];
  const material = useMemo<ResearchMaterial[]>(() => {
    const children = findFolderChildren(tree, nodeId) ?? [];
    return children.map((node) => ({
      id: node.id,
      title: node.artifactPreview?.title ?? node.title,
      kind: node.artifactPreview?.kind ?? node.kind,
      isContainer: isContainerKind(node.kind),
      updatedLabel: node.artifactPreview?.updatedLabel,
    }));
  }, [tree, nodeId]);

  return { folder, material, loading, error, saving, onSaveHypothesis };
}
