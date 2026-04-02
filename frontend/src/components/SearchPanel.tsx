/**
 * SearchPanel — semantic search over the knowledge base.
 * Replaces the basic text-filter search in the sidebar.
 */
import { useState, useRef } from 'react'
import type { SearchResult } from '../types'
import { semanticSearch } from '../lib/api'

const TYPE_ICON: Record<string, string> = {
  audio: '🎙', link: '🔗', text: '📋', file: '📎',
  feed: '📡', post: '💬', note: '📝', webpage: '🌐',
}

interface Props {
  onOpenArtifact: (artifact: SearchResult) => void
}

export default function SearchPanel({ onOpenArtifact }: Props) {
  const [query,   setQuery]   = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState<string | null>(null)
  const [searched, setSearched] = useState(false)

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleChange = (value: string) => {
    setQuery(value)
    setError(null)

    if (debounceRef.current) clearTimeout(debounceRef.current)
    if (!value.trim()) { setResults([]); setSearched(false); return }

    debounceRef.current = setTimeout(async () => {
      setLoading(true)
      try {
        const { results: r } = await semanticSearch(value, { limit: 20 })
        setResults(r)
        setSearched(true)
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Search failed')
      } finally {
        setLoading(false)
      }
    }, 500)
  }

  return (
    <div className="flex flex-col h-full">

      {/* Search input */}
      <div className="px-4 py-4 border-b border-gray-100 shrink-0">
        <div className="relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 text-sm">⌕</span>
          <input
            type="text"
            value={query}
            onChange={e => handleChange(e.target.value)}
            placeholder="Search your knowledge…"
            className="w-full pl-8 pr-4 py-2 rounded-lg border border-gray-200 bg-gray-50 text-sm outline-none focus:border-gray-400 focus:bg-white transition-colors"
            autoFocus
          />
        </div>
        {query && (
          <p className="text-[10px] text-gray-400 mt-1.5 px-1">
            Semantic search — finds related ideas, not just keyword matches
          </p>
        )}
      </div>

      {/* Results */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-24 text-gray-300 text-xs">Searching…</div>
        ) : error ? (
          <div className="px-4 py-4">
            <p className="text-xs text-red-400">{error}</p>
          </div>
        ) : !searched ? (
          <div className="flex flex-col items-center justify-center h-full text-gray-300 gap-2 px-4">
            <span className="text-3xl">🔍</span>
            <p className="text-xs text-center">Type to search your full knowledge base semantically</p>
          </div>
        ) : results.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-32 text-gray-300 gap-2 px-4">
            <p className="text-xs text-center">No results found for "{query}"</p>
            <p className="text-[10px] text-center">Try broader terms or check if artifacts are embedded yet</p>
          </div>
        ) : (
          <div className="p-2 space-y-0.5">
            {results.map(r => (
              <button
                key={r.id}
                onClick={() => onOpenArtifact(r)}
                className="w-full text-left px-3 py-3 rounded-lg hover:bg-gray-100 transition-colors group"
              >
                <div className="flex items-start gap-2">
                  <span className="text-sm shrink-0 mt-0.5">{TYPE_ICON[r.type] ?? '◻'}</span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-medium text-gray-800 leading-snug line-clamp-2 group-hover:text-black">
                      {r.label}
                    </p>
                    {r.enrichment?.summary && (
                      <p className="text-[10px] text-gray-400 mt-0.5 line-clamp-2 leading-relaxed">
                        {r.enrichment.summary}
                      </p>
                    )}
                    <div className="flex items-center gap-2 mt-1">
                      <div
                        className="h-1 rounded-full bg-blue-400"
                        style={{ width: `${Math.round(r.similarity * 100)}%`, maxWidth: '60px' }}
                      />
                      <span className="text-[9px] text-gray-300">{Math.round(r.similarity * 100)}% match</span>
                    </div>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
