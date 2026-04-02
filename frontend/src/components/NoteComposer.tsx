/**
 * NoteComposer — floating modal for creating notes.
 * Surfaces related artifacts from your knowledge base while you type
 * using semantic search (no save needed — purely based on live content).
 */
import { useState, useEffect, useRef } from 'react'
import type { Artifact, SearchResult } from '../types'
import { createNote, semanticSearch } from '../lib/api'

const TYPE_ICON: Record<string, string> = {
  audio: '🎙', link: '🔗', text: '📋', file: '📎',
  feed: '📡', post: '💬', note: '📝', webpage: '🌐',
}

interface Props {
  onSave:   (note: Artifact) => void
  onCancel: () => void
}

export default function NoteComposer({ onSave, onCancel }: Props) {
  const [label,   setLabel]   = useState('')
  const [content, setContent] = useState('')
  const [saving,  setSaving]  = useState(false)
  const [related, setRelated] = useState<SearchResult[]>([])

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Live semantic search as the user types (debounced 600ms)
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current)
    const query = (label + ' ' + content).trim()
    if (query.length < 10) { setRelated([]); return }

    debounceRef.current = setTimeout(async () => {
      try {
        const { results } = await semanticSearch(query, { limit: 6, threshold: 0.3 })
        setRelated(results)
      } catch { /* embedding may not be ready */ }
    }, 600)

    return () => { if (debounceRef.current) clearTimeout(debounceRef.current) }
  }, [label, content])

  const handleSave = async () => {
    if (!content.trim() || saving) return
    setSaving(true)
    try {
      const note = await createNote(label, content)
      onSave(note)
    } catch (e) {
      console.error('Failed to save note', e)
    } finally {
      setSaving(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); handleSave() }
    if (e.key === 'Escape') onCancel()
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/20 backdrop-blur-sm"
      onClick={e => { if (e.target === e.currentTarget) onCancel() }}
    >
      <div className="bg-white rounded-2xl shadow-2xl w-full max-w-3xl mx-4 flex overflow-hidden" style={{ maxHeight: '80vh' }}>

        {/* Editor */}
        <div className="flex-1 flex flex-col min-w-0">
          <div className="flex items-center gap-3 px-6 py-4 border-b border-gray-100">
            <span className="text-lg">📝</span>
            <input
              type="text"
              value={label}
              onChange={e => setLabel(e.target.value)}
              placeholder="Title (optional)"
              className="flex-1 text-sm font-medium text-gray-900 placeholder-gray-300 outline-none bg-transparent"
              autoFocus
            />
            <button onClick={onCancel} className="text-gray-300 hover:text-gray-600 text-lg leading-none">×</button>
          </div>

          <textarea
            value={content}
            onChange={e => setContent(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Write your note… (⌘↵ to save)"
            className="flex-1 px-6 py-4 text-sm text-gray-700 leading-relaxed resize-none outline-none placeholder-gray-300"
          />

          <div className="flex items-center justify-between px-6 py-3 border-t border-gray-100 bg-gray-50 shrink-0">
            <span className="text-xs text-gray-300">{content.length} chars</span>
            <div className="flex gap-2">
              <button
                onClick={onCancel}
                className="px-4 py-2 rounded-lg text-xs text-gray-500 hover:bg-gray-100 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                disabled={!content.trim() || saving}
                className="px-4 py-2 rounded-lg bg-gray-900 text-white text-xs font-medium hover:bg-gray-700 disabled:opacity-30 transition-colors"
              >
                {saving ? 'Saving…' : 'Save note'}
              </button>
            </div>
          </div>
        </div>

        {/* Related artifacts sidebar — live semantic search */}
        <div className="w-60 shrink-0 border-l border-gray-100 flex flex-col bg-gray-50">
          <div className="px-4 py-3 border-b border-gray-100 shrink-0">
            <p className="text-[10px] uppercase tracking-widest text-gray-400">From your collection</p>
          </div>
          <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
            {related.length === 0 ? (
              <div className="flex flex-col items-center justify-center h-24 text-gray-300 gap-1.5 px-2">
                <p className="text-[10px] text-center">Related items appear as you write</p>
              </div>
            ) : (
              related.map(a => (
                <div
                  key={a.id}
                  className="flex items-start gap-2 px-3 py-2.5 rounded-lg hover:bg-white transition-colors"
                >
                  <span className="text-sm shrink-0 mt-0.5">{TYPE_ICON[a.type] ?? '◻'}</span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs text-gray-700 leading-snug line-clamp-2">{a.label}</p>
                    {a.enrichment?.topics?.[0] && (
                      <p className="text-[10px] text-gray-400 mt-0.5">{a.enrichment.topics[0]}</p>
                    )}
                    <div
                      className="h-0.5 rounded-full bg-blue-300 mt-1"
                      style={{ width: `${Math.round(a.similarity * 100)}%` }}
                    />
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
