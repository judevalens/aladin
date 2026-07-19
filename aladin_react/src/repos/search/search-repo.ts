import type { ApiClient } from "@/shared/api/client";
import type { SearchResponse } from "@/shared/api/models";

const EMPTY: SearchResponse = { sections: [] };

export interface SearchRepo {
  /** The global command-box search. Empty query → no request, empty result. */
  query(q: string, limit?: number): Promise<SearchResponse>;
}

export function createSearchRepo(client: ApiClient): SearchRepo {
  return {
    query: (q, limit = 6) => {
      const trimmed = q.trim();
      if (!trimmed) return Promise.resolve(EMPTY);
      const params = new URLSearchParams({ q: trimmed, limit: String(limit) });
      return client
        .fetch<SearchResponse>(`/api/search?${params.toString()}`)
        .then((r) => r ?? EMPTY);
    },
  };
}
