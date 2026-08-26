import { ApiError, type ApiClient } from "@/shared/api/client";
import type { LocalReadingPosition } from "@/repos/reading-position/local-reading-position-types";

/** The server's committed row for a position report. */
export interface ReadingPositionDto {
  artifactId: string;
  page: number;
  seq: number;
  updatedAt: number; // unix ms
}

export interface ReadingPositionRepo {
  /** The stored position, or null when none exists (web fallback read). */
  get(artifactId: string): Promise<LocalReadingPosition | null>;
  /** Report the current page (last-write-wins); returns the committed row. */
  put(artifactId: string, page: number): Promise<ReadingPositionDto>;
}

export function createReadingPositionRepo(client: ApiClient): ReadingPositionRepo {
  return {
    get: (artifactId) =>
      client
        .fetch<ReadingPositionDto>(`/api/reading-positions/${encodeURIComponent(artifactId)}`)
        .then((dto): LocalReadingPosition | null => ({
          id: dto.artifactId,
          page: dto.page,
          updatedAt: dto.updatedAt,
        }))
        .catch((error: unknown) => {
          if (error instanceof ApiError && error.status === 404) return null;
          throw error;
        }),

    put: (artifactId, page) =>
      client.fetch<ReadingPositionDto>(
        `/api/reading-positions/${encodeURIComponent(artifactId)}`,
        { method: "PUT", body: JSON.stringify({ page }) },
      ),
  };
}
