# Aladin — Data Models

> Historical note: this document predates the global source item refactor and
> still uses the old `Source → Snapshot → Artifact` ingestion vocabulary.
> For current live-source backend storage, use
> [`docs/GLOBAL_SOURCE_ITEM_PIPELINE.md`](../docs/GLOBAL_SOURCE_ITEM_PIPELINE.md).
> The current ingestion chain is:
>
> `provider_streams → source_items → source_item_enrichments → tenant_item_matches → records`
>
> `records` is tenant-derived output and no longer carries `source_id`; source
> context is resolved through `tenant_item_matches`.

## Vocabulary

| Term | Definition |
|---|---|
| **Source** | Where data lives. A system, account, or file. Configured once, synced over time. |
| **Snapshot** | A point-in-time capture of a source. One source produces many snapshots. |
| **Artifact** | A discrete unit of content extracted from a snapshot. The atomic unit of meaning. |
| **Node** | Either a whole artifact or an extracted entity promoted into the knowledge graph. |
| **Edge** | A typed, temporal relationship between two nodes, supported by artifact evidence. |
| **Insight** | A pipeline-derived observation. Proposes new nodes/edges or flags changes to existing ones. |
| **Knowledge Graph (KG)** | A named, namespaced graph owned by a user. A user can have multiple KGs. |

---

## The Chain

```
Source → Snapshot → Artifact → Node → Edge
                                  ↖       ↑
                                   Insight─┘
```

---

## Node Types

There are exactly two kinds of nodes:

**Artifact node** — the whole artifact promoted into the graph. Links back to `artifact_id` and inherits its content, summary, and metadata. One artifact = one artifact node.

**Entity node** — something *extracted from* artifacts. A person, organization, concept, topic, or event. Does not map 1:1 to a single artifact — aggregates evidence from many. No `artifact_id`.

```
Artifact node ──MENTIONS──► Entity node
Artifact node ──SUPPORTS──► Entity node
Entity node   ──RELATES_TO─► Entity node
```

---

## Artifact Promotion Paths

An artifact lives in Postgres until it is promoted into the graph as a node. There are three promotion paths:

| Path | Trigger | Ghost? | User action needed? |
|---|---|---|---|
| **Manual** | User clicks "add to graph" | No | No — user already decided |
| **Auto** | Pipeline, confidence ≥ `auto_promote_threshold` | No | No — pipeline trusted |
| **Suggested** | Pipeline, confidence ≥ `suggest_threshold` | Yes | Accept or dismiss |

Artifacts below `suggest_threshold` are dismissed and never surface to the user.

### Pipeline Autonomy
Controlled per KG. Determines how aggressively the pipeline promotes artifacts:

```
KnowledgeGraph.pipeline_autonomy
  'suggest' → pipeline always creates ghost nodes, user reviews everything
  'auto'    → pipeline auto-promotes above threshold, ghosts below
  'off'     → manual only — pipeline enriches but never touches the graph
```

---

## Pipeline Stages

Three distinct stages with different triggers and cost profiles:

| Stage | Trigger | Granularity | Cost |
|---|---|---|---|
| **Embedding** | Artifact created | Per-artifact, immediate | Cheap — unblocks search right away |
| **Enrichment** | Embedding done | Per-artifact, async | Medium — LLM for summary/entities/topics |
| **Insight generation** | Snapshot complete (live sources) or artifact enriched (manual) | Batch (live) / Immediate (manual) | Expensive — ANN search + selective LLM |

Manual input (`sync_mode: 'one_shot'`) triggers insight generation immediately after enrichment — the user is present and waiting. Live sources (`sync_mode: 'poll' | 'push'`) batch insight generation at snapshot completion.

---

## Snapshot Versioning & Insight Replay

Every insight references a specific `trigger_artifact_id` pointing to an artifact at a specific `snapshot_id` + `version`. This creates a complete audit trail — the graph's full evolution is recoverable.

**Replay:** filter nodes/edges by `created_at ≤ snapshot.created_at`, exclude edges invalidated before that point.

**Trend detection:** entity nodes accumulate evidence across snapshots. Plotting `mention_count` over snapshot versions reveals concepts rising or fading.

**Counterfactuals:** replay the graph without applying a specific insight's `proposed_changes`.

**Drift detection:** compare graph state at snapshot v1 vs vN — which edges survived, which were invalidated, which clusters merged.

The snapshot version is a **vector clock for your knowledge**. Each sync advances the clock and the graph state is recoverable at any tick.

---

## Models

### Source
Where data comes from. Not content — a pointer to content.

```
Source
  - id
  - user_id
  - kg_id → KnowledgeGraph       ← one source feeds one KG (multi-KG is future)
  - name: string
  - type: 'reddit' | 'bluesky' | 'slack' | 'email' | 'file' | 'url' | 'manual'
  - sync_mode: 'poll' | 'push' | 'one_shot'
  - sync_state: 'pending' | 'active' | 'paused' | 'error'
  - config: JSONB                 ← credentials, cursors, filters — shape varies by type
  - auto_promote_threshold: float ← above this → auto Node (no review)
  - suggest_threshold: float      ← above this → ghost Node, below → dismissed
  - next_sync_at: timestamp       ← when scheduler should next enqueue jobs (poll only)
  - last_synced_at: timestamp
  - created_at
```

**Notes:**
- `sync_mode: 'poll'` → scheduler-driven, cursor in config, uses SyncJob queue
- `sync_mode: 'push'` → webhook-driven, signing secret in config, bypasses queue
- `sync_mode: 'one_shot'` → manual input, single snapshot, never re-synced
- Credentials in `config` must be encrypted at rest
- Thresholds are per-source — a curated PDF may have a lower bar than a noisy Slack channel

---

### Snapshot
A point-in-time capture of a source. The unit of sync.

```
Snapshot
  - id
  - source_id → Source
  - version: int                ← incrementing per source, starts at 1
  - status: 'processing' | 'complete' | 'failed'
  - expected_jobs: int          ← how many sync jobs were enqueued for this snapshot
  - completed_jobs: int         ← incremented by worker on each job completion
  - artifact_count: int
  - metadata: JSONB             ← sync range, record count, errors, duration
  - created_at
```

**Notes:**
- Snapshot is created when `plan()` runs (before jobs execute). Status starts `processing`.
- `expected_jobs` is set at creation. Worker increments `completed_jobs` on each job completion.
- When `completed_jobs = expected_jobs` → status transitions to `complete` → insight batch enqueued.
- A failed snapshot leaves existing artifacts untouched.
- One-shot sources produce a single snapshot (version: 1). Re-upload creates version 2.

---

### Artifact
A discrete unit of content extracted from a snapshot. This is what gets embedded, enriched, and reasoned over.

```
Artifact
  - id
  - source_id → Source
  - snapshot_id → Snapshot
  - external_id: string         ← stable ID from source system (Reddit fullname, Slack msg ID)
  - version: int                ← increments when same external_id changes across snapshots
  - superseded_by → Artifact    ← nullable, points to newer version of same external_id
  - type: 'document_chunk' | 'message' | 'email' | 'transcript_segment' | 'note' | 'webpage'
  - content: text
  - embedding: vector           ← pgvector, generated immediately on creation
  - metadata: JSONB             ← author, timestamp, url, channel, subject, etc.
  - enrichment: JSONB           ← summary, entities, topics, key_claims, sentiment
  - relevance_score: float      ← computed against KG at enrichment time (null if KG empty)
  - status: 'pending' | 'embedded' | 'enriched' | 'in_graph' | 'dismissed' | 'superseded'
  - created_at
```

**True identity of a data point:** `(source_id, external_id, version)`

**Notes:**
- `status` transitions: pending → embedded → enriched → (in_graph | dismissed | superseded)
- `relevance_score` is null when KG has no nodes (cold start). Cold start rule: if KG has fewer than 10 nodes, skip relevance filtering and enrich everything.
- Artifacts below `source.suggest_threshold` are marked `dismissed` after enrichment.
- When `external_id` reappears with changed content: old artifact → `superseded`, new artifact created with `version + 1`.

---

### KnowledgeGraph
A named, namespaced graph. Users can have multiple KGs (e.g. one per project, one per class).

```
KnowledgeGraph
  - id
  - user_id
  - name: string
  - description: string
  - pipeline_autonomy: 'suggest' | 'auto' | 'off'
  - created_at
```

---

### Node
Either a whole artifact or an extracted entity promoted into a KG. Lives in Neo4j.

```
Node
  - id
  - kg_id → KnowledgeGraph
  - type: 'artifact' | 'entity'
  - subtype: string             ← entity: 'person' | 'organization' | 'concept' | 'topic' | 'event'
                                ← artifact: mirrors Artifact.type
  - label: string
  - artifact_id → Artifact      ← set for artifact nodes, null for entity nodes
  - evidence: JSONB             ← [{artifact_id, snapshot_id, version}] — for entity nodes
  - properties: JSONB           ← type-specific attributes
  - confidence: float           ← pipeline confidence in this node's validity
  - status: 'active' | 'ghost' | 'archived'
  - promoted_by: 'user' | 'pipeline'
  - x, y: float                 ← last known layout position
  - created_at
```

**Notes:**
- `ghost` nodes are pipeline suggestions pending user acceptance.
- Artifact nodes: one artifact = one node, `artifact_id` is set.
- Entity nodes: aggregated from many artifacts, `evidence` array grows over time.
- Node embeddings are NOT stored in Neo4j — use Artifact.embedding in Postgres for semantic search.

---

### Edge
A typed, temporal relationship between two nodes. Lives in Neo4j.

```
Edge
  - id
  - kg_id → KnowledgeGraph
  - source_node_id → Node
  - target_node_id → Node
  - type: 'supports' | 'contradicts' | 'relates_to' | 'mentions' | 'derived_from' | 'authored_by' | 'part_of'
  - weight: float               ← relationship strength / confidence
  - evidence: JSONB             ← [{artifact_id, snapshot_id, version}]
  - valid_from: timestamp
  - valid_until: timestamp      ← null means still valid. Set when evidence is superseded.
  - status: 'active' | 'ghost' | 'invalidated'
  - promoted_by: 'user' | 'pipeline'
  - created_at
```

**Notes:**
- Edges are never deleted — they are `invalidated` and a new edge replaces them. History is preserved.
- `valid_until` + `invalidated` is the belief revision hook. When an artifact is superseded, edges citing it as evidence are re-evaluated.
- `ghost` edges are pipeline suggestions pending user acceptance.

---

### Insight
A pipeline-derived observation surfaced to the user. The output of belief revision and pattern detection.

```
Insight
  - id
  - kg_id → KnowledgeGraph
  - type: 'reinforcement' | 'contradiction' | 'bridge' | 'obsolescence' | 'extension' | 'convergence'
  - description: string         ← human-readable explanation
  - trigger_artifact_id → Artifact
  - trigger_snapshot_id → Snapshot    ← which sync produced this insight
  - affected_nodes: JSONB       ← [node_id, ...]
  - affected_edges: JSONB       ← [edge_id, ...]
  - proposed_changes: JSONB     ← what the pipeline wants to do if accepted
  - confidence: float           ← pipeline confidence in this insight
  - status: 'pending' | 'accepted' | 'dismissed'
  - created_at
```

**Insight types:**

| Type | Meaning | Detection method | MVP? |
|---|---|---|---|
| `reinforcement` | New data supports an existing relationship | Embedding similarity + shared entities | Yes |
| `extension` | New data adds detail to an existing node | Embedding similarity + new entities | Yes |
| `bridge` | New data connects two previously unconnected clusters | ANN + cluster membership check | Yes |
| `convergence` | Multiple independent sources signal the same node in one batch | Count artifacts per node per snapshot, cross-source | Yes |
| `contradiction` | New data conflicts with an existing relationship | High similarity + LLM key_claims comparison | No — post-MVP |
| `obsolescence` | New data suggests an existing node is outdated | Temporal language detection + LLM | No — post-MVP |

**`proposed_changes` shape:**
```json
{
  "create_nodes": [...],
  "create_edges": [...],
  "invalidate_edges": [...],
  "update_node_properties": [...],
  "add_evidence": [{"edge_id": "...", "artifact_id": "..."}]
}
```

---

## Pipeline Flow

```
Source configured
  → Sync triggered (scheduler for poll, webhook for push, user action for one_shot)
  → Snapshot created (version++, status: processing, expected_jobs set)
  → Jobs enqueued → Worker fetches content → Artifacts created (status: pending)

  → Stage 1: Embedding worker
      → embedding generated immediately
      → status: embedded

  → Stage 2: Enrichment worker
      → summary, entities, topics, key_claims extracted
      → relevance_score computed (skip if KG has < 10 nodes)
      → status: enriched
      → snapshot.completed_jobs++
      → if completed_jobs = expected_jobs → snapshot.status = complete

  → Stage 3: Insight generation
      → one_shot source → run immediately after enrichment
      → live source → triggered by snapshot completion, runs as batch

      Batch pass:
        → ANN search: new artifact embeddings vs existing node embeddings
        → Heuristic detection: reinforcement, extension, bridge, convergence
        → LLM called only for candidates needing reasoning (post-MVP: contradiction, obsolescence)
        → Insights created (status: pending)
        → Apply based on kg.pipeline_autonomy:
            auto   → create nodes/edges directly (promoted_by: pipeline)
            suggest → create ghost nodes/edges, surface insight to user
            off    → no graph changes, insights still created for user review
```

---

## Special Cases

### Manual Input
Direct user input (paste text, voice memo, drag-and-drop file):

```
Source(type: 'manual', sync_mode: 'one_shot')
  → Snapshot(version: 1)
  → Artifact(external_id: generated)
  → embed → enrich → insight immediately
  → Node (promoted_by: 'user' when added to graph)
```

### File Re-upload
Creates a new Snapshot (version increments). Diff logic runs against previous snapshot's artifacts. Changed chunks supersede old ones and trigger edge re-evaluation.

### Cold Start
When KG has fewer than 10 nodes, `relevance_score` is skipped and all enriched artifacts are surfaced. Threshold filtering activates once the KG has enough nodes to provide meaningful signal.

### Multi-KG
A user can maintain separate KGs per context. Each source feeds exactly one KG (for now). Relevance pipeline and autonomy settings run per KG.

### Insight Replay
Every Insight references `trigger_snapshot_id`:
- Scrub to any snapshot version to see the graph at that point
- Trace why any edge exists by following its `evidence` back through snapshots
- Run counterfactuals by replaying without specific insights applied
- Audit dismissed insights — the system may have been right
