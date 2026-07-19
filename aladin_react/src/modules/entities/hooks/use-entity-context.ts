import { useCallback, useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type {
  CompanyFacts,
  ContextItem,
  DataPoint,
  Edge,
  Entity,
  ExternalId,
  PendingMerge,
} from "@/modules/entities/entity-context-types";

export interface UseEntityContext {
  entity: Entity | null;
  edges: Edge[];
  context: ContextItem[];
  /** The judge's pending identity questions ("Same, or just similar?"). */
  merges: PendingMerge[];
  /** Typed data points, hard identity keys, and (company only) the extension facts. */
  dataPoints: DataPoint[];
  externalIds: ExternalId[];
  company: CompanyFacts | null;
  loading: boolean;
  error: string | null;
  /** Draw a connection: writes a YOURS edge, then reloads so the graph visibly grows. */
  drawEdge: (edge: { rel: string; toId: string; why: string }) => Promise<void>;
  /** Fold two entities together (reversibly). */
  acceptMerge: (mergeId: string) => Promise<void>;
  /** Record that they're distinct — negative evidence; the pair is never re-proposed. */
  rejectMerge: (mergeId: string) => Promise<void>;
}

// Loads one entity's Context surface (identity + typed edges + verbatim material) and
// exposes the edge write. A merged-away id resolves server-side to its canonical root,
// so stale links still land somewhere real.
export function useEntityContext(entityId: string): UseEntityContext {
  const { repos } = useAppComposition();
  const [entity, setEntity] = useState<Entity | null>(null);
  const [edges, setEdges] = useState<Edge[]>([]);
  const [context, setContext] = useState<ContextItem[]>([]);
  const [merges, setMerges] = useState<PendingMerge[]>([]);
  const [dataPoints, setDataPoints] = useState<DataPoint[]>([]);
  const [externalIds, setExternalIds] = useState<ExternalId[]>([]);
  const [company, setCompany] = useState<CompanyFacts | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    repos.graphPane
      .getEntityContext(entityId)
      .then((result) => {
        if (cancelled) return;
        setEntity(result.entity);
        setEdges(result.edges);
        setContext(result.context);
        setMerges(result.merges);
        setDataPoints(result.dataPoints ?? []);
        setExternalIds(result.externalIds ?? []);
        setCompany(result.company ?? null);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load entity");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos, entityId, nonce]);

  const drawEdge = useCallback(
    async (edge: { rel: string; toId: string; why: string }) => {
      await repos.graphPane.drawEntityEdge(entityId, edge);
      setNonce((n) => n + 1);
    },
    [repos, entityId],
  );

  // A merge decision changes what this entity IS — accepting one can even redirect this
  // page to the entity it folded into — so both outcomes refetch rather than patch state.
  const acceptMerge = useCallback(
    async (mergeId: string) => {
      await repos.graphPane.acceptEntityMerge(entityId, mergeId);
      setNonce((n) => n + 1);
    },
    [repos, entityId],
  );

  const rejectMerge = useCallback(
    async (mergeId: string) => {
      await repos.graphPane.rejectEntityMerge(entityId, mergeId);
      setNonce((n) => n + 1);
    },
    [repos, entityId],
  );

  return {
    entity, edges, context, merges, dataPoints, externalIds, company,
    loading, error, drawEdge, acceptMerge, rejectMerge,
  };
}
