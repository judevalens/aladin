import type { Artifact, ArtifactType } from '../types'
import { removeArtifact } from '../lib/api'

const ARTIFACT_ICONS: Record<ArtifactType, string> = {
  audio: '🎙️',
  link: '🔗',
  text: '📋',
  file: '📎',
}

const ARTIFACT_COLORS: Record<ArtifactType, string> = {
  audio: 'bg-rose-50 border-rose-200 text-rose-700',
  link: 'bg-blue-50 border-blue-200 text-blue-700',
  text: 'bg-amber-50 border-amber-200 text-amber-700',
  file: 'bg-emerald-50 border-emerald-200 text-emerald-700',
}

const ARTIFACT_HEX: Record<ArtifactType, string> = {
  audio: '#f43f5e',
  link: '#3b82f6',
  text: '#f59e0b',
  file: '#10b981',
}

interface Props {
  artifacts: Artifact[]
  selectedArtifactId: string | null
  onSelectArtifact: (id: string | null) => void
  onAddToGraph: (artifact: Artifact) => void
  onDeleteArtifact: (id: string) => void
}

function ArtifactDetail({
  artifact,
  onBack,
  onAddToGraph,
  onDelete,
}: {
  artifact: Artifact
  onBack: () => void
  onAddToGraph: (a: Artifact) => void
  onDelete: (id: string) => void
}) {
  const color = ARTIFACT_HEX[artifact.type]

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-200 flex items-center gap-2 shrink-0">
        <button
          onClick={onBack}
          className="p-1 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
        </button>
        <h2 className="text-sm font-semibold text-gray-700 truncate flex-1">{artifact.label}</h2>
      </div>

      {/* Type badge */}
      <div className="px-4 pt-4 shrink-0">
        <span
          className="inline-flex items-center gap-1.5 text-xs font-medium text-white px-2 py-1 rounded-full"
          style={{ background: color }}
        >
          {ARTIFACT_ICONS[artifact.type]}
          {artifact.type.charAt(0).toUpperCase() + artifact.type.slice(1)}
        </span>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-4 py-4">
        <p className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-2">Content</p>
        {artifact.type === 'audio' ? (
          <audio controls src={artifact.content} className="w-full mt-1" />
        ) : (
          <p className="text-sm text-gray-700 whitespace-pre-wrap break-words leading-relaxed">
            {artifact.content}
          </p>
        )}

        {artifact.sourceUrl && (
          <a
            href={artifact.sourceUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-3 inline-flex items-center gap-1 text-xs text-blue-500 hover:underline break-all"
          >
            ↗ {artifact.sourceUrl}
          </a>
        )}

        {artifact.enrichment && (
          <div className="mt-4 flex flex-col gap-3">
            {artifact.enrichment.topics.length > 0 && (
              <div>
                <p className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-1.5">Topics</p>
                <div className="flex flex-wrap gap-1">
                  {artifact.enrichment.topics.map((t) => (
                    <span key={t} className="text-xs px-2 py-0.5 rounded-full bg-gray-100 text-gray-600">{t}</span>
                  ))}
                </div>
              </div>
            )}
            {artifact.enrichment.key_claims.length > 0 && (
              <div>
                <p className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-1.5">Key claims</p>
                <ul className="flex flex-col gap-1">
                  {artifact.enrichment.key_claims.map((c) => (
                    <li key={c} className="text-xs text-gray-600 flex gap-1.5">
                      <span className="text-gray-300 shrink-0">—</span>{c}
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {artifact.enrichment.entities.length > 0 && (
              <div>
                <p className="text-xs font-medium text-gray-400 uppercase tracking-wide mb-1.5">Entities</p>
                <div className="flex flex-wrap gap-1">
                  {artifact.enrichment.entities.map((e) => (
                    <span key={e} className="text-xs px-2 py-0.5 rounded-full bg-gray-50 border border-gray-200 text-gray-500">{e}</span>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-3 border-t border-gray-100 flex flex-col gap-2 shrink-0">
        <button
          onClick={() => onAddToGraph(artifact)}
          className="w-full text-xs py-1.5 px-2 rounded font-medium text-white transition-opacity hover:opacity-90"
          style={{ background: color }}
        >
          Add to graph
        </button>
        <button
          onClick={() => onDelete(artifact.id)}
          className="w-full text-xs py-1.5 px-2 rounded font-medium text-red-500 border border-red-200 hover:bg-red-50 transition-colors"
        >
          Delete artifact
        </button>
      </div>
    </div>
  )
}

export default function CollectionPanel({ artifacts, selectedArtifactId, onSelectArtifact, onAddToGraph, onDeleteArtifact }: Props) {
  const selected = selectedArtifactId
    ? artifacts.find((a) => a.id === selectedArtifactId) ?? null
    : null

  const handleDelete = (id: string) => {
    removeArtifact(id).catch(console.error)
    onDeleteArtifact(id)
    onSelectArtifact(null)
  }

  if (selected) {
    return (
      <div className="w-64 border-r border-gray-200 flex flex-col bg-white shrink-0">
        <ArtifactDetail
          artifact={selected}
          onBack={() => onSelectArtifact(null)}
          onAddToGraph={onAddToGraph}
          onDelete={handleDelete}
        />
      </div>
    )
  }

  return (
    <div className="w-64 border-r border-gray-200 flex flex-col bg-white shrink-0">
      <div className="px-4 py-3 border-b border-gray-200">
        <h2 className="text-sm font-semibold text-gray-700">Collection</h2>
        <p className="text-xs text-gray-400 mt-0.5">
          {artifacts.length} artifact{artifacts.length !== 1 ? 's' : ''}
        </p>
      </div>

      <div className="flex-1 overflow-y-auto p-3 flex flex-col gap-2">
        {artifacts.length === 0 && (
          <p className="text-xs text-gray-400 text-center mt-8">
            Right-click the canvas to add artifacts
          </p>
        )}
        {artifacts.map((artifact) => (
          <button
            key={artifact.id}
            onClick={() => onSelectArtifact(artifact.id)}
            className={`rounded-lg border p-3 flex flex-col gap-2 text-left w-full transition-opacity hover:opacity-80 ${ARTIFACT_COLORS[artifact.type]}`}
          >
            <div className="flex items-start gap-2">
              <span className="text-base leading-none mt-0.5">{ARTIFACT_ICONS[artifact.type]}</span>
              <div className="flex-1 min-w-0">
                <p className="text-xs font-medium truncate">{artifact.label}</p>
                <p className="text-xs opacity-60 truncate mt-0.5">{artifact.content}</p>
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
