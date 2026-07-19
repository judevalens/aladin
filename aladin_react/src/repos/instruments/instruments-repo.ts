import type { ApiClient } from "@/shared/api/client";
import type { InstrumentHit } from "@/shared/api/models";

export interface InstrumentsRepo {
  /** Ticker typeahead for the command box. Empty query → no request, empty result. */
  search(query: string, limit?: number): Promise<InstrumentHit[]>;
}

export function createInstrumentsRepo(client: ApiClient): InstrumentsRepo {
  return {
    search: (query, limit = 20) => {
      const q = query.trim();
      if (!q) return Promise.resolve([]);
      const params = new URLSearchParams({ q, limit: String(limit) });
      return client
        .fetch<InstrumentHit[]>(`/api/instruments/search?${params.toString()}`)
        .then((hits) => hits ?? []);
    },
  };
}
