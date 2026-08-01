import type { ApiClient } from "@/shared/api/client";

/**
 * An artifact's ingested content (design/INGESTION_PRD.md).
 *
 * `status` is always present and always meaningful — §4: extraction fails in ways worth
 * telling apart, and a surface should never have to choose between "still working" and
 * "quietly gave up".
 */
export type DocumentStatus = "pending" | "ingesting" | "ready" | "unsupported" | "failed";

export interface DocumentSection {
  title: string;
  level: number;
  page: number;
}

export interface DocumentPage {
  page: number;
  text: string;
}

export interface IngestedDocument {
  artifactId: string;
  status: DocumentStatus;
  error?: string;
  pageCount: number;
  sections: DocumentSection[];
  pages?: DocumentPage[];
  extractor?: string;
}

export interface DocumentRepo {
  /** `withText` is opt-in: a book is megabytes, and a status chip doesn't need the words. */
  get(artifactId: string, withText: boolean): Promise<IngestedDocument | null>;
}

export function createDocumentRepo(client: ApiClient): DocumentRepo {
  return {
    async get(artifactId, withText) {
      const query = withText ? "?text=1" : "";
      try {
        return await client.fetch<IngestedDocument>(
          `/api/artifacts/${encodeURIComponent(artifactId)}/document${query}`,
        );
      } catch (error) {
        // 404 is the ordinary state for a note, a link, or a file nothing extracts —
        // "no document" is not an error worth surfacing as one.
        if (error instanceof Error && /404|not found/i.test(error.message)) return null;
        throw error;
      }
    },
  };
}
