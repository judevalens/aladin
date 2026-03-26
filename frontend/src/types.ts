export type PaneType = 'graph' | 'documents' | 'live-data'

export interface PaneData {
  id: string
  type: PaneType
}

export type ArtifactType = 'audio' | 'link' | 'text' | 'file'

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
  createdAt: Date
}
