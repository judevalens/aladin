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
    throw new Error(err.error ?? `Ingest failed (${res.status})`)
  }
  return res.json()
}

export async function fetchArtifacts(): Promise<import('../types').Artifact[]> {
  const res = await fetch('/api/artifacts/')
  if (!res.ok) throw new Error(`Failed to load artifacts (${res.status})`)
  const data = await res.json()
  return data.map((a: import('../types').Artifact & { createdAt: string }) => ({
    ...a,
    createdAt: new Date(a.createdAt),
  }))
}

export async function fetchChildren(
  parentId: string,
  opts: { limit?: number; offset?: number } = {},
): Promise<{ items: import('../types').Artifact[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.limit)  params.set('limit',  String(opts.limit))
  if (opts.offset) params.set('offset', String(opts.offset))
  const res = await fetch(`/api/artifacts/${parentId}/children?${params}`)
  if (!res.ok) throw new Error(`Failed to load children (${res.status})`)
  const data = await res.json()
  return {
    items: data.items.map((a: import('../types').Artifact & { createdAt: string }) => ({
      ...a,
      createdAt: new Date(a.createdAt),
    })),
    total: data.total,
  }
}

export async function persistArtifact(artifact: import('../types').Artifact): Promise<void> {
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

export async function enrichArtifact(id: string): Promise<import('../types').ArtifactEnrichment> {
  const res = await fetch(`/api/pipeline/enrich/${id}`, { method: 'POST' })
  if (!res.ok) throw new Error(`Enrichment failed (${res.status})`)
  const data = await res.json()
  return data.enrichment
}

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

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

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
        const { token } = JSON.parse(payload)
        if (token) yield token
      } catch { /* skip malformed */ }
    }
  }
}

export async function uploadDocument(file: File): Promise<void> {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch('/api/documents/upload', { method: 'POST', body: form })
  if (!res.ok) {
    const err = await res.json().catch(() => ({}))
    throw new Error(err.error ?? `Upload failed (${res.status})`)
  }
}
