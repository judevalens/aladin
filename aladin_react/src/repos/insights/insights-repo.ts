import type { ApiClient } from "@/shared/api/client";
import type { Insight } from "@/modules/insights/insight-types";

export interface InsightsRepo {
  list(status: string, type?: string): Promise<Insight[]>;
  accept(id: string): Promise<void>;
  dismiss(id: string): Promise<void>;
}

export function createInsightsRepo(client: ApiClient): InsightsRepo {
  return {
    list: (status, type) => {
      const params = new URLSearchParams({ status, limit: "100" });
      if (type) params.set("type", type);
      // Backend wraps the list: {items, total, limit, offset}. Unwrap to the array.
      return client
        .fetch<{ items: Insight[] }>(`/api/insights/?${params.toString()}`)
        .then((out) => out.items ?? []);
    },
    accept: (id) =>
      client
        .fetch<{ ok: boolean }>(`/api/insights/${encodeURIComponent(id)}/accept`, { method: "POST" })
        .then(() => undefined),
    dismiss: (id) =>
      client
        .fetch<{ ok: boolean }>(`/api/insights/${encodeURIComponent(id)}/dismiss`, { method: "POST" })
        .then(() => undefined),
  };
}
