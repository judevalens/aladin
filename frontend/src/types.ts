export type PaneType = 'graph' | 'documents' | 'live-data'

export interface PaneData {
  id: string
  type: PaneType
}

export type ArtifactType = 'audio' | 'link' | 'text' | 'file' | 'feed' | 'post' | 'note' | 'comment' | 'chunk' | 'webpage'

export interface ArtifactEnrichment {
  summary: string
  entities: string[]
  key_claims: string[]
  topics: string[]
}

export interface Artifact {
  id: string
  type: ArtifactType
  label: string
  content: string
  sourceUrl?: string
  enrichment?: ArtifactEnrichment
  metadata?: Record<string, unknown>
  childCount?: number
  userStatus?: 'saved' | 'dismissed' | null
  status?: string
  createdAt: Date
}

export type InsightType = 'bridge' | 'convergence' | 'trend' | 'contradiction'

export interface Insight {
  id: string
  type: InsightType
  title: string
  body: string
  entity?: string
  topic?: string
  artifactIds: string[]
  confidence: number
  userStatus: 'pending' | 'accepted' | 'dismissed'
  createdAt: Date
}

export interface FeedItem extends Artifact {
  sourceType?: string
  sourceName?: string
  signalScore?: number
}

export interface SearchResult extends Artifact {
  similarity: number
}
