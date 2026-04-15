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

export type SourceKind =
  | 'reddit_subreddit'
  | 'bluesky_search'
  | 'bluesky_account'
  | 'twitter_search'

export interface SourceRecord {
  id: string
  name: string
  type: string
  syncMode: string
  syncState: string
  config: Record<string, unknown>
  autoPromoteThreshold: number
  suggestThreshold: number
  createdAt: string
  lastSyncedAt?: string | null
}

export interface CreateSourceInput {
  kind: SourceKind
  name?: string
  subreddit?: string
  minScore?: number
  includeComments?: boolean
  topComments?: number
  sort?: 'new' | 'hot' | 'top'
  query?: string
  handle?: string
  limit?: number
  minLikes?: number
  maxResults?: number
}
