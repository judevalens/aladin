import { useState } from 'react'
import type { CreateSourceInput, SourceKind, SourceRecord } from '../types'

const SOURCE_OPTIONS: { kind: SourceKind; label: string; description: string }[] = [
  { kind: 'reddit_subreddit', label: 'Reddit subreddit', description: 'Track new posts from one subreddit.' },
  { kind: 'bluesky_search', label: 'Bluesky search', description: 'Follow a recurring Bluesky query.' },
  { kind: 'bluesky_account', label: 'Bluesky account', description: 'Watch posts from one Bluesky handle.' },
  { kind: 'twitter_search', label: 'X search', description: 'Persist a Twitter/X search feed.' },
]

type FormState = {
  kind: SourceKind
  name: string
  subreddit: string
  minScore: number
  includeComments: boolean
  topComments: number
  sort: 'new' | 'hot' | 'top'
  query: string
  handle: string
  limit: number
  minLikes: number
  maxResults: number
}

const INITIAL_FORM: FormState = {
  kind: 'reddit_subreddit',
  name: '',
  subreddit: '',
  minScore: 10,
  includeComments: true,
  topComments: 5,
  sort: 'new',
  query: '',
  handle: '',
  limit: 50,
  minLikes: 5,
  maxResults: 25,
}

interface Props {
  sources: SourceRecord[]
  onCreateSource: (payload: CreateSourceInput) => Promise<void>
  onDeleteSource: (id: string) => Promise<void>
}

export default function SourceManager({ sources, onCreateSource, onDeleteSource }: Props) {
  const [form, setForm] = useState<FormState>(INITIAL_FORM)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [removingId, setRemovingId] = useState<string | null>(null)

  const selectedOption = SOURCE_OPTIONS.find((option) => option.kind === form.kind) ?? SOURCE_OPTIONS[0]

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setSubmitting(true)
    setError(null)

    try {
      const payload: CreateSourceInput = {
        kind: form.kind,
        name: form.name.trim() || undefined,
      }

      if (form.kind === 'reddit_subreddit') {
        payload.subreddit = form.subreddit.trim()
        payload.minScore = form.minScore
        payload.includeComments = form.includeComments
        payload.topComments = form.topComments
        payload.sort = form.sort
      }

      if (form.kind === 'bluesky_search') {
        payload.query = form.query.trim()
        payload.limit = form.limit
      }

      if (form.kind === 'bluesky_account') {
        payload.handle = form.handle.trim()
        payload.limit = form.limit
      }

      if (form.kind === 'twitter_search') {
        payload.query = form.query.trim()
        payload.minLikes = form.minLikes
        payload.maxResults = form.maxResults
      }

      await onCreateSource(payload)
      setForm((prev) => ({
        ...prev,
        name: '',
        subreddit: '',
        query: '',
        handle: '',
      }))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create source')
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    setRemovingId(id)
    setError(null)
    try {
      await onDeleteSource(id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete source')
    } finally {
      setRemovingId(null)
    }
  }

  return (
    <aside className="w-[24rem] shrink-0 border-l border-gray-200 bg-white/80 backdrop-blur-sm">
      <div className="flex h-full flex-col">
        <div className="border-b border-gray-200 px-5 py-4">
          <h3 className="text-sm font-semibold text-gray-900">Sources</h3>
          <p className="mt-1 text-xs leading-relaxed text-gray-500">
            Create persistent feeds to follow across Reddit, Bluesky, and X.
          </p>
        </div>

        <div className="flex-1 space-y-5 overflow-y-auto px-5 py-5">
          <form onSubmit={handleSubmit} className="space-y-4 rounded-2xl border border-gray-200 bg-gray-50 p-4">
            <div>
              <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.16em] text-gray-500">
                Source Type
              </label>
              <select
                value={form.kind}
                onChange={(event) => setForm((prev) => ({ ...prev, kind: event.target.value as SourceKind }))}
                className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 outline-none focus:border-gray-400"
              >
                {SOURCE_OPTIONS.map((option) => (
                  <option key={option.kind} value={option.kind}>{option.label}</option>
                ))}
              </select>
              <p className="mt-2 text-xs text-gray-500">{selectedOption.description}</p>
            </div>

            <Field
              label="Display name"
              placeholder="Optional custom label"
              value={form.name}
              onChange={(value) => setForm((prev) => ({ ...prev, name: value }))}
            />

            {form.kind === 'reddit_subreddit' && (
              <>
                <Field
                  label="Subreddit"
                  placeholder="MachineLearning"
                  value={form.subreddit}
                  onChange={(value) => setForm((prev) => ({ ...prev, subreddit: value }))}
                />
                <div className="grid grid-cols-2 gap-3">
                  <NumberField
                    label="Min score"
                    value={form.minScore}
                    onChange={(value) => setForm((prev) => ({ ...prev, minScore: value }))}
                  />
                  <NumberField
                    label="Top comments"
                    value={form.topComments}
                    onChange={(value) => setForm((prev) => ({ ...prev, topComments: value }))}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.16em] text-gray-500">
                      Sort
                    </label>
                    <select
                      value={form.sort}
                      onChange={(event) => setForm((prev) => ({ ...prev, sort: event.target.value as FormState['sort'] }))}
                      className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 outline-none focus:border-gray-400"
                    >
                      <option value="new">New</option>
                      <option value="hot">Hot</option>
                      <option value="top">Top</option>
                    </select>
                  </div>
                  <label className="mt-6 flex items-center gap-2 rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700">
                    <input
                      type="checkbox"
                      checked={form.includeComments}
                      onChange={(event) => setForm((prev) => ({ ...prev, includeComments: event.target.checked }))}
                    />
                    Include comments
                  </label>
                </div>
              </>
            )}

            {form.kind === 'bluesky_search' && (
              <>
                <Field
                  label="Search query"
                  placeholder="open source intelligence"
                  value={form.query}
                  onChange={(value) => setForm((prev) => ({ ...prev, query: value }))}
                />
                <NumberField
                  label="Items per sync"
                  value={form.limit}
                  onChange={(value) => setForm((prev) => ({ ...prev, limit: value }))}
                />
              </>
            )}

            {form.kind === 'bluesky_account' && (
              <>
                <Field
                  label="Handle"
                  placeholder="alice.bsky.social"
                  value={form.handle}
                  onChange={(value) => setForm((prev) => ({ ...prev, handle: value }))}
                />
                <NumberField
                  label="Items per sync"
                  value={form.limit}
                  onChange={(value) => setForm((prev) => ({ ...prev, limit: value }))}
                />
              </>
            )}

            {form.kind === 'twitter_search' && (
              <>
                <Field
                  label="Search query"
                  placeholder="knowledge graph OR vector db"
                  value={form.query}
                  onChange={(value) => setForm((prev) => ({ ...prev, query: value }))}
                />
                <div className="grid grid-cols-2 gap-3">
                  <NumberField
                    label="Min likes"
                    value={form.minLikes}
                    onChange={(value) => setForm((prev) => ({ ...prev, minLikes: value }))}
                  />
                  <NumberField
                    label="Max results"
                    value={form.maxResults}
                    onChange={(value) => setForm((prev) => ({ ...prev, maxResults: value }))}
                  />
                </div>
              </>
            )}

            {error && (
              <div className="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                {error}
              </div>
            )}

            <button
              type="submit"
              disabled={submitting}
              className="w-full rounded-xl bg-gray-900 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-black disabled:cursor-not-allowed disabled:opacity-50"
            >
              {submitting ? 'Saving source…' : 'Add source'}
            </button>
          </form>

          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-semibold uppercase tracking-[0.16em] text-gray-500">Tracked feeds</h4>
              <span className="text-xs text-gray-400">{sources.length}</span>
            </div>

            {sources.length === 0 ? (
              <div className="rounded-2xl border border-dashed border-gray-200 px-4 py-6 text-center text-sm text-gray-400">
                No saved sources yet.
              </div>
            ) : (
              <div className="space-y-3">
                {sources.map((source) => (
                  <div key={source.id} className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-medium text-gray-900">{source.name}</p>
                        <p className="mt-1 text-xs text-gray-500">{describeSource(source)}</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => handleDelete(source.id)}
                        disabled={removingId === source.id}
                        className="rounded-lg px-2 py-1 text-xs text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 disabled:opacity-50"
                      >
                        {removingId === source.id ? 'Removing…' : 'Remove'}
                      </button>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-gray-500">
                      <span className="rounded-full bg-gray-100 px-2 py-1">{source.type}</span>
                      <span className="rounded-full bg-gray-100 px-2 py-1">{source.syncState}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </aside>
  )
}

function Field({
  label,
  placeholder,
  value,
  onChange,
}: {
  label: string
  placeholder: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <div>
      <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.16em] text-gray-500">
        {label}
      </label>
      <input
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 outline-none focus:border-gray-400"
      />
    </div>
  )
}

function NumberField({
  label,
  value,
  onChange,
}: {
  label: string
  value: number
  onChange: (value: number) => void
}) {
  return (
    <div>
      <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.16em] text-gray-500">
        {label}
      </label>
      <input
        type="number"
        value={value}
        onChange={(event) => onChange(Number(event.target.value))}
        className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-700 outline-none focus:border-gray-400"
      />
    </div>
  )
}

function describeSource(source: SourceRecord): string {
  const config = source.config ?? {}
  if (source.type === 'reddit') {
    return `r/${String(config.subreddit ?? '')} · ${String(config.sort ?? 'new')}`
  }
  if (source.type === 'bluesky' && config.mode === 'search') {
    return `search: ${String(config.query ?? '')}`
  }
  if (source.type === 'bluesky' && config.mode === 'account') {
    return `@${String(config.handle ?? '')}`
  }
  if (source.type === 'twitter') {
    return `search: ${String(config.query ?? '')}`
  }
  return source.type
}
