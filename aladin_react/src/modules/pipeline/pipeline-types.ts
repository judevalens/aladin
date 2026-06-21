// Mirrors GET /api/pipeline/stats (backend SystemRepository.PipelineStats).

export interface PipelineStats {
  records: {
    byStatus: Record<string, number>;
    inFlight: number;
    stuckOver15Min: number;
    failed: number;
    oldestInFlightSecs: number | null;
  };
  throughput: {
    completedLast1h: number;
    completedLast24h: number;
    avgLatencySecs: number | null;
  };
  insights: {
    byType: Record<string, number>;
    byStatus: Record<string, number>;
  };
  matches: { byStatus: Record<string, number> };
  relationships: number;
}
