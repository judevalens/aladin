import { useEffect, useState } from 'react'
import { fetchInsightStats, fetchWorkerStatus } from '../lib/api'

interface Props {
  activePanel: string | null
  onTogglePanel: (panel: string) => void
}

const NAV_ITEMS = [
  {
    id: 'collection',
    title: 'Collection',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
        d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
    ),
  },
  {
    id: 'documents',
    title: 'Documents',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
    ),
  },
  {
    id: 'search',
    title: 'Search',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
        d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
    ),
  },
  {
    id: 'insights',
    title: 'Insights',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
        d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
    ),
  },
  {
    id: 'graph-explorer',
    title: 'Graph Explorer',
    icon: (
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
        d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
    ),
  },
]

export default function NavSidebar({ activePanel, onTogglePanel }: Props) {
  const [insightCount, setInsightCount] = useState(0)
  const [pipelineBusy, setPipelineBusy] = useState(false)

  // Poll insight count + pipeline status every 30s
  useEffect(() => {
    const poll = () => {
      fetchInsightStats().then(s => {
        setInsightCount(s.byStatus['pending'] ?? 0)
      }).catch(() => {})
      fetchWorkerStatus().then(s => {
        setPipelineBusy(s.queue.pendingPipeline > 0)
      }).catch(() => {})
    }
    poll()
    const id = setInterval(poll, 30_000)
    return () => clearInterval(id)
  }, [])

  return (
    <div className="w-12 border-r border-gray-200 flex flex-col items-center py-3 gap-1 shrink-0">
      {NAV_ITEMS.map((item) => (
        <div key={item.id} className="relative">
          <button
            title={item.title}
            onClick={() => onTogglePanel(item.id)}
            className={`w-8 h-8 rounded-lg flex items-center justify-center transition-colors ${
              activePanel === item.id
                ? 'bg-gray-900 text-white'
                : 'text-gray-400 hover:bg-gray-100 hover:text-gray-600'
            }`}
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              {item.icon}
            </svg>
          </button>
          {/* Insight badge */}
          {item.id === 'insights' && insightCount > 0 && (
            <span className="absolute -top-0.5 -right-0.5 w-3.5 h-3.5 rounded-full bg-blue-500 text-white text-[8px] font-bold flex items-center justify-center leading-none">
              {insightCount > 9 ? '9+' : insightCount}
            </span>
          )}
        </div>
      ))}

      {/* Pipeline indicator (bottom) */}
      <div className="mt-auto mb-1" title={pipelineBusy ? 'Pipeline processing…' : 'Pipeline idle'}>
        <div className={`w-1.5 h-1.5 rounded-full transition-colors ${
          pipelineBusy ? 'bg-emerald-400 animate-pulse' : 'bg-gray-200'
        }`} />
      </div>
    </div>
  )
}
