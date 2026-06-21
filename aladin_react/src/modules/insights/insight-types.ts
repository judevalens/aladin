// Mirrors GET /api/insights/ (backend service.InsightRecord).

export interface Insight {
  id: string;
  type: string; // trend | discourse
  title: string;
  body: string;
  entity: string | null;
  topic: string | null;
  recordIds: string[];
  confidence: number;
  userStatus: string; // pending | accepted | dismissed
  createdAt: string;

  // Discourse insights (type='discourse') carry these.
  entityId?: string | null;
  trustTier?: string | null;
  version?: number;
  provenance?: unknown;
}
