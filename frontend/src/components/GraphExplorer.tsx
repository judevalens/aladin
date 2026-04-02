/**
 * GraphExplorer — sidebar panel for exploring the knowledge graph.
 * Shows top entities + topics, and lets you open entity pages.
 */
import { useState, useEffect } from 'react'
import { fetchGraphEntities, fetchGraphTopics, fetchEntityPage } from '../lib/api'

interface EntityRow  { name: string; artifact_count: number }
interface ArtifactRef { id: string; label: string; type: string }

interface EntityPageData {
  entity:    string
  artifacts: ArtifactRef[]
  neighbors: { name: string; shared_artifacts: number }[]
}

interface Props {
  onSelectArtifact?: (id: string) => void
}

type Tab = 'entities' | 'topics'

export default function GraphExplorer({ onSelectArtifact }: Props) {
  const [tab,        setTab]        = useState<Tab>('entities')
  const [entities,   setEntities]   = useState<EntityRow[]>([])
  const [topics,     setTopics]     = useState<EntityRow[]>([])
  const [loading,    setLoading]    = useState(true)
  const [entityPage, setEntityPage] = useState<EntityPageData | null>(null)
  const [pageLoading, setPageLoading] = useState(false)

  useEffect(() => {
    setLoading(true)
    Promise.all([
      fetchGraphEntities(50),
      fetchGraphTopics(30),
    ]).then(([e, t]) => {
      setEntities(e)
      setTopics(t)
    }).catch(console.error)
    .finally(() => setLoading(false))
  }, [])

  const openEntity = async (name: string) => {
    setPageLoading(true)
    try {
      const data = await fetchEntityPage(name)
      setEntityPage(data)
    } catch { /* ignore */ }
    finally { setPageLoading(false) }
  }

  if (entityPage) {
    return (
      <EntityPageView
        data={entityPage}
        onBack={() => setEntityPage(null)}
        onSelectArtifact={onSelectArtifact}
        onOpenEntity={openEntity}
      />
    )
  }

  const rows = tab === 'entities' ? entities : topics

  return (
    <div className="flex flex-col h-full">
      <div className="px-5 py-4 border-b border-gray-100 shrink-0">
        <h3 className="text-sm font-semibold text-gray-900 mb-3">Knowledge Graph</h3>
        <div className="flex rounded-lg border border-gray-200 overflow-hidden">
          {(['entities', 'topics'] as Tab[]).map(t => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`flex-1 py-1.5 text-xs font-medium transition-colors capitalize ${
                tab === t ? 'bg-gray-900 text-white' : 'text-gray-500 hover:bg-gray-50'
              }`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-24 text-gray-300 text-xs">Loading…</div>
        ) : rows.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 text-gray-300 gap-2 px-4">
            <p className="text-xs text-center">
              {tab === 'entities'
                ? 'No entities yet. They appear after artifacts are enriched.'
                : 'No topics yet.'}
            </p>
          </div>
        ) : (
          <div className="p-2 space-y-0.5">
            {rows.map(row => (
              <button
                key={row.name}
                onClick={() => tab === 'entities' ? openEntity(row.name) : undefined}
                className={`w-full flex items-center justify-between px-3 py-2.5 rounded-lg transition-colors text-left ${
                  tab === 'entities' ? 'hover:bg-gray-100 cursor-pointer' : 'cursor-default'
                }`}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className={`w-2 h-2 rounded-full shrink-0 ${
                    tab === 'entities' ? 'bg-emerald-400' : 'bg-amber-400'
                  }`} />
                  <span className="text-xs text-gray-700 truncate">{row.name}</span>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <span className="text-[10px] text-gray-400">{row.artifact_count}</span>
                  {tab === 'entities' && (
                    <span className="text-gray-300 text-xs">→</span>
                  )}
                </div>
              </button>
            ))}
          </div>
        )}
        {pageLoading && (
          <div className="flex items-center justify-center h-12 text-gray-300 text-xs">Loading…</div>
        )}
      </div>
    </div>
  )
}

// ── EntityPageView ─────────────────────────────────────────────────────────

function EntityPageView({
  data,
  onBack,
  onSelectArtifact,
  onOpenEntity,
}: {
  data: EntityPageData
  onBack: () => void
  onSelectArtifact?: (id: string) => void
  onOpenEntity: (name: string) => void
}) {
  return (
    <div className="flex flex-col h-full">
      <div className="px-5 py-4 border-b border-gray-100 shrink-0">
        <button
          onClick={onBack}
          className="text-xs text-gray-400 hover:text-gray-700 mb-3 flex items-center gap-1 transition-colors"
        >
          ← Entities
        </button>
        <div className="flex items-center gap-2">
          <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 shrink-0" />
          <h3 className="text-sm font-semibold text-gray-900 truncate">{data.entity}</h3>
        </div>
        <p className="text-xs text-gray-400 mt-1">{data.artifacts.length} artifacts</p>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {/* Artifacts mentioning this entity */}
        {data.artifacts.length > 0 && (
          <div>
            <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Mentions</p>
            <div className="space-y-0.5">
              {data.artifacts.map(a => (
                <button
                  key={a.id}
                  onClick={() => onSelectArtifact?.(a.id)}
                  className="w-full text-left px-2 py-2 rounded-lg hover:bg-gray-100 transition-colors"
                >
                  <p className="text-xs text-gray-700 line-clamp-1">{a.label}</p>
                  <p className="text-[10px] text-gray-400">{a.type}</p>
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Neighboring entities */}
        {data.neighbors.length > 0 && (
          <div>
            <p className="text-[10px] uppercase tracking-widest text-gray-400 mb-2">Co-mentioned entities</p>
            <div className="space-y-0.5">
              {data.neighbors.map(n => (
                <button
                  key={n.name}
                  onClick={() => onOpenEntity(n.name)}
                  className="w-full flex items-center justify-between px-2 py-2 rounded-lg hover:bg-gray-100 transition-colors"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-300 shrink-0" />
                    <span className="text-xs text-gray-600 truncate">{n.name}</span>
                  </div>
                  <span className="text-[10px] text-gray-400 shrink-0">{n.shared_artifacts} shared</span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
