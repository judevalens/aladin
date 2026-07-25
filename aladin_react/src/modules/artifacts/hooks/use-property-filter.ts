import { useCallback, useEffect, useMemo, useState, useSyncExternalStore } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { PropertyFacet } from "@/repos/artifacts/property-query-repo";
import type { Artifact } from "@/shared/api/models";

export interface UsePropertyFilter {
  /** The property keys in use, with their distinct values. */
  facets: PropertyFacet[];
  facetsLoading: boolean;
  key: string;
  value: string;
  select: (key: string, value?: string) => void;
  clear: () => void;
  /** Artifacts matching the current filter (undefined while loading, [] when no key is picked). */
  results: Artifact[] | undefined;
}

/**
 * H1c — filter artifacts by a typed property. Results come from a server-side query (the whole
 * workspace, not just the cached subset) that RE-RUNS on node DataEvents, so the view stays live as
 * properties are edited — no invalidation signal, per the reactive-via-syncer rule.
 */
export function usePropertyFilter(): UsePropertyFilter {
  const { repos } = useAppComposition();
  const [facets, setFacets] = useState<PropertyFacet[]>([]);
  const [facetsLoading, setFacetsLoading] = useState(true);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");

  useEffect(() => {
    let cancelled = false;
    setFacetsLoading(true);
    repos.propertyQuery
      .facets()
      .then((f) => {
        if (!cancelled) setFacets(f);
      })
      .catch(() => {
        if (!cancelled) setFacets([]);
      })
      .finally(() => {
        if (!cancelled) setFacetsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [repos]);

  // A stable no-op store while no key is selected keeps the hook order fixed (no conditional hooks)
  // and avoids firing a query with an empty key (the server rejects it).
  const store = useMemo(
    () =>
      key
        ? repos.propertyQuery.observe(key, value)
        : { subscribe: () => () => {}, snapshot: () => [] as Artifact[] },
    [repos, key, value],
  );
  const results = useSyncExternalStore(store.subscribe, store.snapshot);

  const select = useCallback((nextKey: string, nextValue = "") => {
    setKey(nextKey);
    setValue(nextValue);
  }, []);
  const clear = useCallback(() => {
    setKey("");
    setValue("");
  }, []);

  return { facets, facetsLoading, key, value, select, clear, results };
}
