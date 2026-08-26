import { useEffect, useMemo } from "react";
import { EMPTY, type Observable } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import {
  createPositionReporter,
  type PositionReporter,
} from "@/modules/documents/domain/position-reporter";
import type { ReadingPositionState } from "@/services/reading-position/reading-position-service";
import type { Result } from "@/shared/flow/result";
import { useObservableState } from "@/shared/flow/use-observable-state";

/**
 * One document's synced position, read where the reader renders (per-key, like
 * `useArtifact`). `loaded` distinguishes "still resolving" from "no position":
 * the writer must NOT arm until loaded — see position-reporter.ts. A failed load
 * counts as loaded (page null) so the writer isn't disarmed forever.
 */
export function useReadingPosition(artifactId: string | null): {
  loaded: boolean;
  page: number | null;
} {
  const { services } = useAppComposition();
  const stream = useMemo<Observable<Result<ReadingPositionState>>>(
    () => (artifactId ? services.readingPositions.byArtifact(artifactId) : EMPTY),
    [services.readingPositions, artifactId],
  );
  const state = useObservableState(stream);
  if (state.status === "data") return { loaded: true, page: state.value.page };
  if (state.status === "error") return { loaded: true, page: null };
  return { loaded: false, page: null };
}

/**
 * The debounced position writer for one document. The caller arms it once the
 * restore has resolved; page changes flow through `notePage`. Flushes on
 * app-hide and unmount so the last page survives a close.
 */
export function useReadingPositionReporter(artifactId: string | null): PositionReporter | null {
  const { services } = useAppComposition();
  const reporter = useMemo(
    () =>
      artifactId
        ? createPositionReporter((page) => services.readingPositions.report(artifactId, page))
        : null,
    [services.readingPositions, artifactId],
  );

  useEffect(() => {
    if (!reporter) return;
    const onVisibility = () => {
      if (document.visibilityState === "hidden") reporter.flush();
    };
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      document.removeEventListener("visibilitychange", onVisibility);
      reporter.flush();
    };
  }, [reporter]);

  return reporter;
}
