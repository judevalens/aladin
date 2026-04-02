/**
 * InsightsPanel — shows AI-generated insights from the knowledge base.
 * Displayed in the NavSidebar or as a full view.
 */
import { useState, useEffect } from 'react'
import type { Insight, InsightType } from '../types'
import { fetchInsights, acceptInsight, dismissInsight, fetchInsightStats } from '../lib/api'

const TYPE_ICON: Record<InsightType, string> = {
  bridge:       '🌉',
  convergence:  '🌀',
  trend:        '📈',
  contradiction: '⚡',
}

const TYPE_COLOR: Record<InsightType, string> = {
  bridge:       'bg-purple-50 border-purple-200 text-purple-800',
  convergence:  'bg-blue-50   border-blue-200   text-blue-800',
  trend:        'bg-amber-50  border-amber-200  text-amber-800',
  contradiction: 'bg-red-50   border-red-200    text-red-800',
}

const TYPE_LABEL: Record<InsightType, string> = {
  bridge:       'Bridge',
  convergence:  'Convergence',
  trend:        'Trend',
  contradiction: 'Contradiction',
}

interface Props {
  onViewArtifacts?: (ids: string[]) => void
}

export default function InsightsPanel({ onViewArtifacts }: Props) {
  const [insights,   setInsights]   = useState<Insight[]>([])
  const [total,      setTotal]      = useState(0)
  const [loading,    setLoading]    = useState(true)
  const [typeFilter, setTypeFilter] = useState<string>('')
  const [stats,      setStats]      = useState<{ byType: Record<string, number>; byStatus: Record<string, number> }>({
    byType: {}, byStatus: {},
  })

  useEffect(() => {
    setLoading(true)
    Promise.all([
      fetchInsights({ limit: 50, type: typeFilter || undefined, status: 'pending' }),
      fetchInsightStats(),
    ]).then(([{ items, total: t }, s]) => {
      setInsights(items)
      setTotal(t)
      setStats(s)
    }).catch(console.error).finally(() => setLoading(false))
  }, [typeFilter])

  const handleAccept = async (insight: Insight) => {
    setInsights(prev => prev.filter(i => i.id !== insight.id))
    setTotal(prev => prev - 1)
    await acceptInsight(insight.id)
  }

  const handleDismiss = async (insight: Insight) => {
    setInsights(prev => prev.filter(i => i.id !== insight.id))
    setTotal(prev => prev - 1)
    await dismissInsight(insight.id)
  }

  const totalPending = stats.byStatus['pending'] ?? 0

  return (
    <div className="flex flex-col h-full">

      {/* Header */}
      <div className="px-5 py-4 border-b border-gray-100 shrink-0">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-gray-900">Insights</h3>
          {totalPending > 0 && (
            <span className="px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 text-[10px] font-semibold">
              {totalPending} new
            </span>
          )}
        </div>

        {/* Type filter tabs */}
        <div className="flex gap-1 flex-wrap">
          {(['', 'bridge', 'convergence', 'trend', 'contradiction'] as const).map(t => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-2.5 py-1 rounded-md text-[11px] font-medium transition-colors ${
                typeFilter === t
                  ? 'bg-gray-900 text-white'
                  : 'text-gray-400 hover:text-gray-700 hover:bg-gray-100'
              }`}
            >
              {t === '' ? 'All' : `${TYPE_ICON[t as InsightType]} ${TYPE_LABEL[t as InsightType]}`}
            </button>
          ))}
        </div>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {loading ? (
          <div className="flex items-center justify-center h-24 text-gray-300 text-xs">Loading…</div>
        ) : insights.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 text-gray-300 gap-2">
            <span className="text-3xl">💡</span>
            <p className="text-xs text-center">No insights yet.<br />They appear after syncing data.</p>
          </div>
        ) : (
          insights.map(insight => (
            <InsightCard
              key={insight.id}
              insight={insight}
              onAccept={() => handleAccept(insight)}
              onDismiss={() => handleDismiss(insight)}
              onViewArtifacts={
                insight.artifactIds.length > 0 && onViewArtifacts
                  ? () => onViewArtifacts(insight.artifactIds)
                  : undefined
              }
            />
          ))
        )}

        {!loading && total > insights.length && (
          <p className="text-center text-xs text-gray-300 py-2">
            +{total - insights.length} more
          </p>
        )}
      </div>
    </div>
  )
}

// ── InsightCard ────────────────────────────────────────────────────────────

function InsightCard({
  insight,
  onAccept,
  onDismiss,
  onViewArtifacts,
}: {
  insight: Insight
  onAccept: () => void
  onDismiss: () => void
  onViewArtifacts?: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  const colorClass = TYPE_COLOR[insight.type] ?? 'bg-gray-50 border-gray-200 text-gray-800'
  const icon       = TYPE_ICON[insight.type] ?? '💡'
  const confidence = Math.round(insight.confidence * 100)

  return (
    <div className={`rounded-xl border p-3 ${colorClass} relative group`}>
      {/* Type badge */}
      <div className="flex items-center gap-1.5 mb-2">
        <span className="text-sm">{icon}</span>
        <span className="text-[10px] font-semibold uppercase tracking-wide opacity-60">
          {TYPE_LABEL[insight.type]}
        </span>
        <span className="ml-auto text-[10px] opacity-40">{confidence}%</span>
      </div>

      {/* Title */}
      <p className="text-xs font-semibold leading-snug mb-1">{insight.title}</p>

      {/* Body (expandable) */}
      {expanded ? (
        <p className="text-xs leading-relaxed opacity-80 mb-2">{insight.body}</p>
      ) : (
        <p className="text-xs leading-relaxed opacity-80 line-clamp-2 mb-2">{insight.body}</p>
      )}
      {insight.body.length > 100 && (
        <button
          onClick={() => setExpanded(v => !v)}
          className="text-[10px] opacity-50 hover:opacity-80 mb-2 transition-opacity"
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}

      {/* Tags */}
      {(insight.entity || insight.topic) && (
        <div className="mb-2">
          <span className="px-1.5 py-0.5 rounded text-[10px] bg-white/60 font-medium">
            {insight.entity ?? insight.topic}
          </span>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center gap-1.5">
        <button
          onClick={onAccept}
          className="flex-1 py-1 rounded-lg text-[11px] font-medium bg-white/60 hover:bg-white/90 transition-colors"
        >
          Accept
        </button>
        {onViewArtifacts && (
          <button
            onClick={onViewArtifacts}
            className="py-1 px-2 rounded-lg text-[11px] bg-white/40 hover:bg-white/70 transition-colors"
            title="View related artifacts"
          >
            {insight.artifactIds.length} artifacts
          </button>
        )}
        <button
          onClick={onDismiss}
          className="py-1 px-2 rounded-lg text-[11px] text-current opacity-40 hover:opacity-70 transition-opacity"
          title="Dismiss"
        >
          ×
        </button>
      </div>

      {/* Age */}
      <p className="text-[10px] opacity-30 mt-2">
        {new Date(insight.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}
      </p>
    </div>
  )
}
