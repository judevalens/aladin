import type { ApiClient } from "@/shared/api/client";
import type { PipelineStats } from "@/modules/pipeline/pipeline-types";

export interface PipelineRepo {
  getStats(): Promise<PipelineStats>;
}

export function createPipelineRepo(client: ApiClient): PipelineRepo {
  return {
    getStats: () => client.fetch<PipelineStats>("/api/pipeline/stats"),
  };
}
