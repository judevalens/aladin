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

/**
 * A node of the recovered chunk tree (INGESTION_PRD §11) — a tree, not a partition, so a
 * section is a node *and* holds nodes. Text is omitted: this is for navigating.
 */
export interface DocumentChunk {
  id: number;
  parentId?: number;
  ordinal: number;
  depth: number;
  kind: "section" | "block";
  title?: string;
  pageFrom: number;
  pageTo: number;
  children?: DocumentChunk[];
}

export interface DocumentRepo {
  /** `withText` is opt-in: a book is megabytes, and a status chip doesn't need the words. */
  get(artifactId: string, withText: boolean): Promise<IngestedDocument | null>;
  /**
   * The structure segmentation recovered, which for most PDFs is an outline the file never
   * carried — the MIT thesis has *zero* embedded bookmarks and 280 pages.
   */
  outline(artifactId: string): Promise<DocumentChunk[]>;
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

    async outline(artifactId) {
      try {
        const result = await client.fetch<{ chunks?: DocumentChunk[] }>(
          `/api/artifacts/${encodeURIComponent(artifactId)}/outline`,
        );
        return result.chunks ?? [];
      } catch (error) {
        // Same reasoning as get(): nothing ingested is an ordinary state, not a failure.
        if (error instanceof Error && /404|not found/i.test(error.message)) return [];
        throw error;
      }
    },
  };
}
