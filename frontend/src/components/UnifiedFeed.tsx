import { useState, useEffect, useCallback, useRef } from 'react'
import type { FeedItem, SourceRecord, CreateSourceInput } from '../types'
import {
  createSource, deleteSource, fetchFeed, fetchFeedTopics, fetchFeedSources, fetchSources,
  saveFeedItem, dismissFeedItem, unsaveFeedItem,
} from '../lib/api'
import SourceManager from './SourceManager'

const SOURCE_ICON: Record<string, string> = {
  reddit:  'r/',
  twitter: '𝕏',
  bluesky: '🦋',
  manual:  '✍',
  file:    '📄',
  url:     '🔗',
  note:    '📋',
}


interface Props {
  onOpenArtifact: (item: FeedItem) => void
}

export default function UnifiedFeed({ onOpenArtifact }: Props) {
  const [items,       setItems]       = useState<FeedItem[]>([])
  const [total,       setTotal]       = useState(0)
  const [loading,     setLoading]     = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [topics,      setTopics]      = useState<string[]>([])
  const [sources,     setSources]     = useState<{ id: string; name: string; type: string }[]>([])
  const [sourceRecords, setSourceRecords] = useState<SourceRecord[]>([])

  // Filters
  const [sort,       setSort]       = useState<'recent' | 'signal'>('recent')
  const [topicFilter, setTopicFilter] = useState<string>('')
  const [sourceFilter, setSourceFilter] = useState<string>('')
  const [savedOnly,  setSavedOnly]  = useState(false)

  const offset = useRef(0)
  const PAGE   = 50

  const load = useCallback(async (reset = false) => {
    if (reset) {
      offset.current = 0
      setItems([])
      setLoading(true)
    } else {
      setLoadingMore(true)
    }
    try {
      const { items: newItems, total: t } = await fetchFeed({
        limit:       PAGE,
        offset:      reset ? 0 : offset.current,
        sort,
        topic:       topicFilter  || undefined,
        source_type: sourceFilter || undefined,
        saved:       savedOnly    || undefined,
      })
      if (reset) {
        setItems(newItems)
      } else {
        setItems(prev => [...prev, ...newItems])
      }
      setTotal(t)
      offset.current = (reset ? 0 : offset.current) + newItems.length
    } catch (e) {
      console.error('Feed load error', e)
    } finally {
      setLoading(false)
      setLoadingMore(false)
    }
  }, [sort, topicFilter, sourceFilter, savedOnly])

  // Initial load + filter changes
  useEffect(() => { load(true) }, [load])

  // Load topics + sources once
  useEffect(() => {
    fetchFeedTopics().then(setTopics).catch(() => {})
    fetchFeedSources().then(setSources).catch(() => {})
    fetchSources().then(setSourceRecords).catch(() => {})
  }, [])

  const refreshSources = useCallback(async () => {
    const [feedSources, fullSources] = await Promise.all([
      fetchFeedSources(),
      fetchSources(),
    ])
    setSources(feedSources)
    setSourceRecords(fullSources)
  }, [])

  const handleCreateSource = useCallback(async (payload: CreateSourceInput) => {
    await createSource(payload)
    await refreshSources()
  }, [refreshSources])

  const handleDeleteSource = useCallback(async (id: string) => {
    await deleteSource(id)
    await refreshSources()
  }, [refreshSources])

  const handleSave = async (e: React.MouseEvent, item: FeedItem) => {
    e.stopPropagation()
    const isSaved = item.userStatus === 'saved'
    setItems(prev => prev.map(i => i.id === item.id
      ? { ...i, userStatus: isSaved ? null : 'saved' }
      : i
    ))
    await (isSaved ? unsaveFeedItem(item.id) : saveFeedItem(item.id))
  }

  const handleDismiss = async (e: React.MouseEvent, item: FeedItem) => {
    e.stopPropagation()
    setItems(prev => prev.filter(i => i.id !== item.id))
    setTotal(prev => prev - 1)
    await dismissFeedItem(item.id)
  }

  const hasMore = items.length < total

  return (
    <div className="flex h-full w-full bg-gray-50">
      <div className="flex min-w-0 flex-1 flex-col">

        {/* Header */}
        <div className="bg-white border-b border-gray-100 px-8 py-5 shrink-0">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-base font-semibold text-gray-900">Unified Feed</h2>
              <p className="text-xs text-gray-400 mt-0.5">{total} items across all sources</p>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setSavedOnly(v => !v)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  savedOnly
                    ? 'bg-amber-50 border-amber-200 text-amber-700'
                    : 'border-gray-200 text-gray-500 hover:bg-gray-50'
                }`}
              >
                ★ Saved
              </button>
              <div className="flex rounded-lg border border-gray-200 overflow-hidden">
                {(['recent', 'signal'] as const).map(s => (
                  <button
                    key={s}
                    onClick={() => setSort(s)}
                    className={`px-3 py-1.5 text-xs font-medium transition-colors capitalize ${
                      sort === s ? 'bg-gray-900 text-white' : 'text-gray-500 hover:bg-gray-50'
                    }`}
                  >
                    {s === 'recent' ? 'Recent' : 'Top'}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* Filter row */}
          <div className="flex items-center gap-2 flex-wrap">
            <select
              value={sourceFilter}
              onChange={e => setSourceFilter(e.target.value)}
              className="px-2.5 py-1.5 rounded-lg border border-gray-200 text-xs bg-white text-gray-600 outline-none focus:border-gray-400"
            >
              <option value="">All sources</option>
              {sources.map(s => (
                <option key={s.id} value={s.type}>{s.name}</option>
              ))}
            </select>

            <select
              value={topicFilter}
              onChange={e => setTopicFilter(e.target.value)}
              className="px-2.5 py-1.5 rounded-lg border border-gray-200 text-xs bg-white text-gray-600 outline-none focus:border-gray-400"
            >
              <option value="">All topics</option>
              {topics.map(t => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>

            {(sourceFilter || topicFilter || savedOnly) && (
              <button
                onClick={() => { setSourceFilter(''); setTopicFilter(''); setSavedOnly(false) }}
                className="px-2.5 py-1.5 rounded-lg text-xs text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
              >
                × Clear filters
              </button>
            )}
          </div>
        </div>

        {/* Feed list */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center h-32 text-gray-300 text-sm">Loading…</div>
          ) : items.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-full text-gray-300 gap-3">
              <span className="text-4xl">◻</span>
              <p className="text-sm">No items match your filters</p>
            </div>
          ) : (
            <>
              <div className="divide-y divide-gray-100">
                {items.map(item => (
                  <FeedRow
                    key={item.id}
                    item={item}
                    onClick={() => onOpenArtifact(item)}
                    onSave={e => handleSave(e, item)}
                    onDismiss={e => handleDismiss(e, item)}
                  />
                ))}
              </div>
              {hasMore && (
                <div className="flex justify-center py-6">
                  <button
                    onClick={() => load(false)}
                    disabled={loadingMore}
                    className="px-6 py-2 rounded-lg border border-gray-200 text-xs text-gray-500 hover:bg-gray-50 disabled:opacity-40 transition-colors"
                  >
                    {loadingMore ? 'Loading…' : `Load more (${total - items.length} remaining)`}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      <SourceManager
        sources={sourceRecords}
        onCreateSource={handleCreateSource}
        onDeleteSource={handleDeleteSource}
      />
    </div>
  )
}

// ── FeedRow ────────────────────────────────────────────────────────────────

function FeedRow({
  item,
  onClick,
  onSave,
  onDismiss,
}: {
  item: FeedItem
  onClick: () => void
  onSave: (e: React.MouseEvent) => void
  onDismiss: (e: React.MouseEvent) => void
}) {
  const score    = (item.metadata?.score    as number) ?? 0
  const comments = (item.metadata?.num_comments as number) ?? 0
  const author   = (item.metadata?.author   as string) ?? ''
  const topics   = item.enrichment?.topics?.slice(0, 3) ?? []
  const summary  = item.enrichment?.summary ?? ''
  const srcIcon  = SOURCE_ICON[item.sourceType ?? ''] ?? '●'
  const isSaved  = item.userStatus === 'saved'

  return (
    <div
      onClick={onClick}
      className="px-8 py-4 hover:bg-white transition-colors group cursor-pointer"
    >
      <div className="flex items-start gap-3">
        {/* Source badge */}
        <div className="w-8 shrink-0 flex flex-col items-center gap-1 mt-0.5">
          <span className="text-xs font-semibold text-gray-400 leading-none">{srcIcon}</span>
          {score > 0 && (
            <span className="text-[10px] text-gray-300 leading-none">
              {score >= 1000 ? `${(score / 1000).toFixed(1)}k` : score}
            </span>
          )}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2 mb-1">
            <p className="text-sm font-medium text-gray-900 group-hover:text-black leading-snug line-clamp-2">
              {item.label}
            </p>
          </div>

          {summary && (
            <p className="text-xs text-gray-500 leading-relaxed line-clamp-2 mb-2">{summary}</p>
          )}

          <div className="flex items-center gap-3 flex-wrap">
            {topics.map(t => (
              <span
                key={t}
                className="px-1.5 py-0.5 rounded bg-gray-100 text-[10px] text-gray-500"
              >{t}</span>
            ))}
            <div className="flex items-center gap-3 ml-auto text-[11px] text-gray-400">
              {author && <span>{author}</span>}
              {comments > 0 && <span>💬 {comments}</span>}
              {item.sourceName && <span>{item.sourceName}</span>}
              <span>{new Date(item.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })}</span>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-1 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            onClick={onSave}
            title={isSaved ? 'Unsave' : 'Save'}
            className={`w-7 h-7 rounded flex items-center justify-center text-xs transition-colors ${
              isSaved
                ? 'text-amber-500 bg-amber-50'
                : 'text-gray-300 hover:text-gray-600 hover:bg-gray-100'
            }`}
          >
            ★
          </button>
          <button
            onClick={onDismiss}
            title="Dismiss"
            className="w-7 h-7 rounded flex items-center justify-center text-xs text-gray-300 hover:text-gray-600 hover:bg-gray-100 transition-colors"
          >
            ×
          </button>
        </div>
      </div>
    </div>
  )
}
