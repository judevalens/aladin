import type { ApiClient } from "@/shared/api/client";
import type { ClaimSignal, SignalSort } from "@/modules/signals/signal-types";

// The "book" lens is served over HTTP (not the local sync cache): it lists the caller's
// authored theses, each marked to market by supporting/contradicting discovered sources.
// (Prototype: bypasses the durable sync path — the inbox lens still rides the local store.)
export interface BookSignalsRepo {
  list(sort: SignalSort): Promise<ClaimSignal[]>;
}

export function createBookSignalsRepo(client: ApiClient): BookSignalsRepo {
  return {
    list: (sort) => {
      const params = new URLSearchParams({ lens: "book", sort, limit: "100" });
      return client
        .fetch<{ signals: ClaimSignal[] }>(`/api/signals?${params.toString()}`)
        .then((out) => out.signals ?? []);
    },
  };
}
