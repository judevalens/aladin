import { useMemo } from "react";
import { EMPTY, type Observable } from "rxjs";

import { useAppComposition } from "@/app/composition/app-composition";
import type { Artifact } from "@/shared/api/models";
import type { Result } from "@/shared/flow/result";
import { useObservableState } from "@/shared/flow/use-observable-state";

/**
 * One artifact, read by whoever is rendering it.
 *
 * This is the per-consumer read: a pane subscribes to its OWN id and to nothing else, so what
 * it shows can't be disturbed by an unrelated tab opening or closing. That works because
 * `KeyedStream` holds the current value per key — subscribing is instant, and two consumers of
 * the same id share one fetch — so "subscribe where you render" costs nothing over passing a
 * shared cache down.
 *
 * `null` means genuinely not loaded yet, not "the aggregate was rebuilt this frame".
 */
export function useArtifact(artifactId: string | null): Artifact | null {
  const { services } = useAppComposition();
  const stream = useMemo<Observable<Result<Artifact>>>(
    () => (artifactId ? services.workspace.artifactById(artifactId) : EMPTY),
    [services.workspace, artifactId],
  );
  const state = useObservableState(stream);
  return state.status === "data" ? state.value : null;
}
