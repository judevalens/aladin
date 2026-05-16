import type { ApiClient } from "@/shared/api/client";
import type { PageDocumentRecord, PageSaveRequest } from "@/shared/api/models";

export interface PageRepo {
  getPage(pageId: string): Promise<PageDocumentRecord>;
  savePage(pageId: string, input: PageSaveRequest): Promise<PageDocumentRecord>;
}

export function createPageRepo(client: ApiClient): PageRepo {
  return {
    getPage: (pageId) => client.fetch<PageDocumentRecord>(`/api/pages/${pageId}`),
    savePage: (pageId, input) =>
      client.fetch<PageDocumentRecord>(`/api/pages/${pageId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
  };
}
