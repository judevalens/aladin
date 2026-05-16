import type { ApiClient } from "@/shared/api/client";
import type { Source, SourceCreateRequest } from "@/shared/api/models";

export interface SourcesRepo {
  getSources(): Promise<Source[]>;
  createSource(request: SourceCreateRequest): Promise<Source>;
  deleteSource(id: string): Promise<void>;
}

export function createSourcesRepo(client: ApiClient): SourcesRepo {
  return {
    getSources: () => client.fetch<Source[]>("/api/sources/"),
    createSource: (request) =>
      client.fetch<Source>("/api/sources/", {
        method: "POST",
        body: JSON.stringify(request),
      }),
    deleteSource: (id) => client.fetch<void>(`/api/sources/${id}`, { method: "DELETE" }),
  };
}
