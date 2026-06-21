import { useEffect, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { PipelineStats } from "@/modules/pipeline/pipeline-types";

export interface UsePipelineStats {
  stats: PipelineStats | null;
  loading: boolean;
  error: string | null;
}

// Polls the pipeline stats endpoint so the panel reflects ingestion in near-real-time.
export function usePipelineStats(intervalMs = 5000): UsePipelineStats {
  const { repos } = useAppComposition();
  const [stats, setStats] = useState<PipelineStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      repos.pipeline
        .getStats()
        .then((result) => {
          if (cancelled) return;
          setStats(result);
          setError(null);
        })
        .catch((err: unknown) => {
          if (cancelled) return;
          setError(err instanceof Error ? err.message : "Failed to load pipeline stats");
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    };
    load();
    const handle = setInterval(load, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(handle);
    };
  }, [repos.pipeline, intervalMs]);

  return { stats, loading, error };
}
