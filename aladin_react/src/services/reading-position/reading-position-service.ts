import { type Observable } from "rxjs";

import type { LocalReadingPositionRepo } from "@/repos/reading-position/local-reading-position-repo";
import type { ReadingPositionRepo } from "@/repos/reading-position/reading-position-repo";
import type { LocalReadingPosition } from "@/repos/reading-position/local-reading-position-types";
import { KeyedStream } from "@/shared/flow/keyed-stream";
import { type Result } from "@/shared/flow/result";

/**
 * One document's synced position — `page: null` means "no position recorded",
 * a legitimate value the reader must distinguish from "not loaded yet".
 */
export interface ReadingPositionState {
  artifactId: string;
  page: number | null;
  /** Server unix-ms stamp; 0 when no position exists. */
  updatedAt: number;
}

/**
 * Reading positions across devices: the per-key read model + the position report.
 *
 * Reads ride the sync replica on desktop (Tauri) and fall back to a one-shot REST
 * GET on the web (a seed at open, not refetch-as-reactivity). Reports go straight
 * to Go (`PUT /api/reading-positions/{id}`, last-write-wins); the committed row is
 * pushed back into the stream — the same value the echo frame will carry, so the
 * frame lands as a no-op. Readers consult the value ONLY at open (apply-at-open):
 * the stream keeps absorbing frames, but nothing yanks the page mid-session.
 */
export class ReadingPositionService {
  private readonly stream = new KeyedStream<string, ReadingPositionState>(
    (state) => state.artifactId,
    (artifactId) => this.fetch(artifactId),
  );

  constructor(
    private readonly rest: ReadingPositionRepo,
    private readonly local: LocalReadingPositionRepo | null,
  ) {}

  byArtifact(artifactId: string): Observable<Result<ReadingPositionState>> {
    return this.stream.observe(artifactId);
  }

  /** Report the current page. Fire-and-forget; a failed report is just lost. */
  report(artifactId: string, page: number) {
    void this.rest
      .put(artifactId, page)
      .then((dto) => {
        this.stream.push({ artifactId: dto.artifactId, page: dto.page, updatedAt: dto.updatedAt });
      })
      .catch((error: unknown) => {
        console.warn("[reading-position] report failed", error);
      });
  }

  /** A frame landed in the replica (desktop live path). */
  handleUpserted(row: LocalReadingPosition) {
    this.stream.push({ artifactId: row.id, page: row.page, updatedAt: row.updatedAt });
  }

  handleDeleted(artifactId: string) {
    this.stream.push({ artifactId, page: null, updatedAt: 0 });
  }

  private async fetch(artifactId: string): Promise<ReadingPositionState> {
    const row = await (this.local ? this.local.get(artifactId) : this.rest.get(artifactId));
    return row
      ? { artifactId, page: row.page, updatedAt: row.updatedAt }
      : { artifactId, page: null, updatedAt: 0 };
  }
}
