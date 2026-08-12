import { useCallback, useEffect, useState } from "react";
import { debounceTime, filter } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import type { DocumentChunk, IngestedDocument } from "@/repos/documents/document-repo";

export interface UseDocument {
  document: IngestedDocument | null;
  loading: boolean;
  error: string | null;
}

/** One row of the flattened outline, ready to render. */
export interface OutlineEntry {
  title: string;
  depth: number;
  page: number;
}

/**
 * Loads an artifact's ingested document (design/INGESTION_PRD.md).
 *
 * Ingestion happens in a background worker, so this surface has to learn about a status
 * change it didn't cause. It rides the syncer for that rather than polling: the worker
 * emits the artifact's node frame when it writes the result, and we refetch off that
 * event — the same shape use-graph-pane and use-research-overview use.
 *
 * `withText` is opt-in so a status chip doesn't pull a book's worth of text.
 */
export function useDocument(artifactId: string, withText: boolean): UseDocument {
  const { repos, runtime } = useAppComposition();
  const [document, setDocument] = useState<IngestedDocument | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    if (!artifactId) return;
    const sub = runtime.dataEvents
      .events()
      .pipe(
        filter((event) => event.type === "nodeUpserted" && event.payload.id === artifactId),
        debounceTime(250),
      )
      .subscribe(() => reload());
    return () => sub.unsubscribe();
  }, [runtime, artifactId, reload]);

  useEffect(() => {
    if (!artifactId) {
      setDocument(null);
      return;
    }
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.documents
      .get(artifactId, withText)
      .then((result) => {
        if (cancelled) return;
        setDocument(result);
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
  }, [repos, artifactId, withText, nonce]);

  return { document, loading, error };
}

/**
 * The outline segmentation RECOVERED (INGESTION_PRD §11), for files that carry none of
 * their own — which is most of them: the MIT thesis has zero embedded bookmarks across
 * 280 pages, so without this a research PDF opens with no way to navigate it.
 *
 * `enabled` is how the caller says "the file's own bookmarks already answered this".
 * Authored structure beats inferred structure (§5), so we only pay for the fallback.
 */
export function useDocumentOutline(artifactId: string, enabled: boolean): OutlineEntry[] {
  const { repos } = useAppComposition();
  const [entries, setEntries] = useState<OutlineEntry[]>([]);

  useEffect(() => {
    if (!artifactId || !enabled) {
      setEntries([]);
      return;
    }
    let cancelled = false;
    repos.documents
      .outline(artifactId)
      .then((chunks) => {
        if (!cancelled) setEntries(flattenOutline(chunks));
      })
      .catch(() => {
        // A missing outline is a missing sidebar, not an error state — the PDF still reads.
        if (!cancelled) setEntries([]);
      });
    return () => {
      cancelled = true;
    };
  }, [repos, artifactId, enabled]);

  return entries;
}

/**
 * Sections only. A `block` is a leaf of body text with no title (§11) — real in the tree,
 * meaningless as a navigation row.
 */
function flattenOutline(chunks: DocumentChunk[]): OutlineEntry[] {
  const out: OutlineEntry[] = [];
  const walk = (nodes: DocumentChunk[]) => {
    for (const node of nodes) {
      if (node.kind === "section" && node.title) {
        out.push({ title: node.title, depth: node.depth, page: node.pageFrom });
      }
      if (node.children?.length) walk(node.children);
    }
  };
  walk(chunks);
  return out;
}
