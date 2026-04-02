import type { Artifact, ArtifactEnrichment, FeedItem, SearchResult, Insight } from '../types'

// ── Graph ──────────────────────────────────────────────────────────────────

export interface GraphNode {
  id: string
  label: string
  type: string
  content: string
  sourceUrl?: string
  x: number
  y: number
  color: string
}

export interface GraphEdge {
  id: string
  source: string
  target: string
}

export async function fetchGraph(): Promise<{ nodes: GraphNode[]; edges: GraphEdge[] }> {
  const res = await fetch('/api/graph')
  if (!res.ok) throw new Error(`Failed to load graph (${res.status})`)
  return res.json()
}

/** Full graph: manual user nodes + pipeline-promoted entities/topics */
export async function fetchFullGraph(): Promise<{ nodes: (GraphNode & { pipeline?: boolean })[]; edges: (GraphEdge & { relType?: string })[] }> {
  const res = await fetch('/api/graph-explore/full')
  if (!res.ok) return { nodes: [], edges: [] }
  return res.json()
}

export async function fetchNeighborhood(
  nodeId: string,
  depth = 1,
): Promise<{ nodes: GraphNode[]; edges: GraphEdge[]; center: string }> {
  const res = await fetch(`/api/graph-explore/neighborhood/${encodeURIComponent(nodeId)}?depth=${depth}`)
  if (!res.ok) return { nodes: [], edges: [], center: nodeId }
  return res.json()
}

export async function fetchGraphEntities(limit = 50): Promise<{ name: string; artifact_count: number }[]> {
  const res = await fetch(`/api/graph-explore/entities?limit=${limit}`)
  if (!res.ok) return []
  return res.json()
}

export async function fetchGraphTopics(limit = 30): Promise<{ name: string; artifact_count: number }[]> {
  const res = await fetch(`/api/graph-explore/topics?limit=${limit}`)
  if (!res.ok) return []
  return res.json()
}

export async function fetchEntityPage(entityName: string): Promise<{
  entity: string
  artifacts: { id: string; label: string; type: string }[]
  neighbors: { name: string; shared_artifacts: number }[]
}> {
  const res = await fetch(`/api/graph-explore/entity/${encodeURIComponent(entityName)}`)
  if (!res.ok) throw new Error(`Entity page failed (${res.status})`)
  return res.json()
}

export async function saveGraphNode(node: GraphNode): Promise<void> {
  const res = await fetch('/api/graph/nodes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(node),
  })
  if (!res.ok) throw new Error(`Failed to save node (${res.status})`)
}

export async function deleteGraphNode(id: string): Promise<void> {
  await fetch(`/api/graph/nodes/${id}`, { method: 'DELETE' })
}

export async function updateNodePosition(id: string, x: number, y: number): Promise<void> {
  await fetch(`/api/graph/nodes/${id}/position`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ x, y }),
  })
}

export async function saveGraphEdge(edge: GraphEdge): Promise<void> {
  const res = await fetch('/api/graph/edges', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(edge),
  })
  if (!res.ok) throw new Error(`Failed to save edge (${res.status})`)
}

// ── Ingest ─────────────────────────────────────────────────────────────────

export interface IngestedPost {
  platform: string
  postId: string
  url: string
  authorHandle: string
  authorName: string
  content: string
  createdAt: string
  likeCount: number
  repostCount: number
  replyCount: number
  embeds: { type: string; url?: string; title?: string; description?: string; alt?: string; uri?: string }[]
  thread: IngestedPost[]
}

export interface IngestedContent {
  type: 'post' | 'article' | 'paper' | 'page'
  rawUrl: string
  post: IngestedPost | null
}

export async function ingestUrl(url: string): Promise<IngestedContent> {
  const res = await fetch('/api/ingest/url', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error((err as { error?: string }).error ?? `Ingest failed (${res.status})`)
  }
  return res.json()
}

// ── Artifacts ──────────────────────────────────────────────────────────────

function parseArtifact(a: Artifact & { createdAt: string }): Artifact {
  return { ...a, createdAt: new Date(a.createdAt) }
}

export async function fetchArtifacts(): Promise<Artifact[]> {
  const res = await fetch('/api/artifacts/')
  if (!res.ok) throw new Error(`Failed to load artifacts (${res.status})`)
  const data = await res.json()
  return data.map(parseArtifact)
}

export async function fetchChildren(
  parentId: string,
  opts: { limit?: number; offset?: number } = {},
): Promise<{ items: Artifact[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.limit)  params.set('limit',  String(opts.limit))
  if (opts.offset) params.set('offset', String(opts.offset))
  const res = await fetch(`/api/artifacts/${parentId}/children?${params}`)
  if (!res.ok) throw new Error(`Failed to load children (${res.status})`)
  const data = await res.json()
  return {
    items: data.items.map(parseArtifact),
    total: data.total,
  }
}

export async function persistArtifact(artifact: Artifact): Promise<void> {
  const res = await fetch('/api/artifacts/', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(artifact),
  })
  if (!res.ok) throw new Error(`Failed to save artifact (${res.status})`)
}

export async function removeArtifact(id: string): Promise<void> {
  await fetch(`/api/artifacts/${id}`, { method: 'DELETE' })
}

export async function enrichArtifact(id: string): Promise<ArtifactEnrichment> {
  const res = await fetch(`/api/pipeline/enrich/${id}`, { method: 'POST' })
  if (!res.ok) throw new Error(`Enrichment failed (${res.status})`)
  const data = await res.json()
  return data.enrichment
}

export async function uploadDocument(file: File): Promise<void> {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch('/api/documents/upload', { method: 'POST', body: form })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error((err as { error?: string }).error ?? `Upload failed (${res.status})`)
  }
}

// ── Feed ───────────────────────────────────────────────────────────────────

export interface FeedFilters {
  limit?: number
  offset?: number
  source_type?: string
  topic?: string
  saved?: boolean
  sort?: 'recent' | 'signal'
}

export async function fetchFeed(filters: FeedFilters = {}): Promise<{ items: FeedItem[]; total: number }> {
  const params = new URLSearchParams()
  if (filters.limit)       params.set('limit',       String(filters.limit))
  if (filters.offset)      params.set('offset',      String(filters.offset))
  if (filters.source_type) params.set('source_type', filters.source_type)
  if (filters.topic)       params.set('topic',       filters.topic)
  if (filters.saved)       params.set('saved',       'true')
  if (filters.sort)        params.set('sort',        filters.sort)

  const res = await fetch(`/api/feed/?${params}`)
  if (!res.ok) throw new Error(`Failed to load feed (${res.status})`)
  const data = await res.json()
  return {
    items: data.items.map((a: FeedItem & { createdAt: string }) => ({ ...a, createdAt: new Date(a.createdAt) })),
    total: data.total,
  }
}

export async function saveFeedItem(id: string): Promise<void> {
  await fetch(`/api/feed/${id}/save`, { method: 'POST' })
}

export async function dismissFeedItem(id: string): Promise<void> {
  await fetch(`/api/feed/${id}/dismiss`, { method: 'POST' })
}

export async function unsaveFeedItem(id: string): Promise<void> {
  await fetch(`/api/feed/${id}/unsave`, { method: 'POST' })
}

export async function fetchFeedTopics(): Promise<string[]> {
  const res = await fetch('/api/feed/topics')
  if (!res.ok) return []
  return res.json()
}

export async function fetchFeedSources(): Promise<{ id: string; name: string; type: string }[]> {
  const res = await fetch('/api/feed/sources')
  if (!res.ok) return []
  return res.json()
}

// ── Search ─────────────────────────────────────────────────────────────────

export async function semanticSearch(
  q: string,
  opts: { limit?: number; source_type?: string; threshold?: number } = {},
): Promise<{ query: string; results: SearchResult[] }> {
  const params = new URLSearchParams({ q })
  if (opts.limit)       params.set('limit',       String(opts.limit))
  if (opts.source_type) params.set('source_type', opts.source_type)
  if (opts.threshold)   params.set('threshold',   String(opts.threshold))

  const res = await fetch(`/api/search/?${params}`)
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error((err as { error?: string }).error ?? `Search failed (${res.status})`)
  }
  const data = await res.json()
  return {
    query:   data.query,
    results: data.results.map((r: SearchResult & { createdAt: string }) => ({
      ...r,
      createdAt: new Date(r.createdAt),
    })),
  }
}

export async function findSimilar(artifactId: string, limit = 8): Promise<SearchResult[]> {
  const res = await fetch('/api/search/similar', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ artifact_id: artifactId, limit }),
  })
  if (!res.ok) return []
  const data = await res.json()
  return data.results.map((r: SearchResult & { createdAt: string }) => ({
    ...r,
    createdAt: new Date(r.createdAt),
  }))
}

// ── Notes ──────────────────────────────────────────────────────────────────

export async function createNote(label: string, content: string): Promise<Artifact> {
  const res = await fetch('/api/notes/', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label, content }),
  })
  if (!res.ok) throw new Error(`Failed to create note (${res.status})`)
  const data = await res.json()
  return { ...data, createdAt: new Date(data.createdAt) }
}

export async function updateNote(id: string, updates: { label?: string; content?: string }): Promise<Artifact> {
  const res = await fetch(`/api/notes/${id}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
  if (!res.ok) throw new Error(`Failed to update note (${res.status})`)
  const data = await res.json()
  return { ...data, createdAt: new Date(data.createdAt) }
}

export async function fetchNoteRelated(id: string): Promise<SearchResult[]> {
  const res = await fetch(`/api/notes/${id}/related`)
  if (!res.ok) return []
  const data = await res.json()
  return (data.results || []).map((r: SearchResult & { createdAt: string }) => ({
    ...r,
    createdAt: new Date(r.createdAt),
  }))
}

// ── Insights ───────────────────────────────────────────────────────────────

export async function fetchInsights(opts: {
  limit?: number
  offset?: number
  type?: string
  status?: string
} = {}): Promise<{ items: Insight[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.limit)  params.set('limit',  String(opts.limit))
  if (opts.offset) params.set('offset', String(opts.offset))
  if (opts.type)   params.set('type',   opts.type)
  if (opts.status) params.set('status', opts.status)

  const res = await fetch(`/api/insights/?${params}`)
  if (!res.ok) throw new Error(`Failed to load insights (${res.status})`)
  const data = await res.json()
  return {
    items: data.items.map((i: Insight & { createdAt: string }) => ({
      ...i,
      createdAt: new Date(i.createdAt),
    })),
    total: data.total,
  }
}

export async function acceptInsight(id: string): Promise<void> {
  await fetch(`/api/insights/${id}/accept`, { method: 'POST' })
}

export async function dismissInsight(id: string): Promise<void> {
  await fetch(`/api/insights/${id}/dismiss`, { method: 'POST' })
}

export async function fetchInsightStats(): Promise<{ byType: Record<string, number>; byStatus: Record<string, number> }> {
  const res = await fetch('/api/insights/stats')
  if (!res.ok) return { byType: {}, byStatus: {} }
  return res.json()
}

// ── Worker Status ──────────────────────────────────────────────────────────

export interface WorkerStatus {
  pipeline: {
    enriched: number
    embedded: number
    promoted: number
    errors: number
    last_tick: string | null
  }
  queue: {
    pending: number
    pendingPipeline: number
    deadJobs: number
  }
}

export async function fetchWorkerStatus(): Promise<WorkerStatus> {
  const res = await fetch('/api/worker/status')
  if (!res.ok) throw new Error(`Failed to fetch worker status (${res.status})`)
  return res.json()
}

// ── Chat ───────────────────────────────────────────────────────────────────

export async function* streamChat(
  message: string,
  history: { role: string; content: string }[],
): AsyncGenerator<string> {
  const res = await fetch('/api/pipeline/chat', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ message, history }),
  })
  if (!res.ok) throw new Error(`Chat failed (${res.status})`)

  const reader  = res.body!.getReader()
  const decoder = new TextDecoder()
  let buffer    = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const lines = buffer.split('\n')
    buffer = lines.pop() ?? ''
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const payload = line.slice(6)
      if (payload === '[DONE]') return
      try {
        const { token } = JSON.parse(payload) as { token?: string }
        if (token) yield token
      } catch { /* skip malformed */ }
    }
  }
}
