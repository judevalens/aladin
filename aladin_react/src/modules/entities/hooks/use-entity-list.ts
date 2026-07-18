import { useCallback, useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type {
  EntityFilter,
  EntityListItem,
  EntitySummary,
} from "@/modules/entities/entity-list-types";

export interface UseEntityList {
  entities: EntityListItem[];
  summary: EntitySummary;
  loading: boolean;
  error: string | null;
  query: string;
  setQuery: (q: string) => void;
  kind: string;
  setKind: (k: string) => void;
  filter: EntityFilter;
  setFilter: (f: EntityFilter) => void;
  /** Refetch — e.g. after a merge decided in the peek changes the counts. */
  reload: () => void;
}

const EMPTY_SUMMARY: EntitySummary = { total: 0, pendingDecisions: 0, tiers: {} };

// Loads the Entities index. The query is debounced and every request is race-guarded, so
// fast typing can't land stale results out of order.
export function useEntityList(): UseEntityList {
  const { repos } = useAppComposition();
  const [query, setQuery] = useState("");
  const [kind, setKind] = useState("");
  const [filter, setFilter] = useState<EntityFilter>("");
  const [entities, setEntities] = useState<EntityListItem[]>([]);
  const [summary, setSummary] = useState<EntitySummary>(EMPTY_SUMMARY);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const reload = useCallback(() => setNonce((n) => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    const timer = setTimeout(() => {
      repos.graphPane
        .listAllEntities({ query, kind, filter })
        .then((result) => {
          if (cancelled) return;
          setEntities(result.entities);
          setSummary(result.summary);
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : "Failed to load entities");
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 150);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [repos, query, kind, filter, nonce]);

  return {
    entities,
    summary,
    loading,
    error,
    query,
    setQuery,
    kind,
    setKind,
    filter,
    setFilter,
    reload,
  };
}
