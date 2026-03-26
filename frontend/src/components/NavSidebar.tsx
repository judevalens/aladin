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
]

export default function NavSidebar({ activePanel, onTogglePanel }: Props) {
  return (
    <div className="w-12 border-r border-gray-200 flex flex-col items-center py-3 gap-1 shrink-0">
      {NAV_ITEMS.map((item) => (
        <button
          key={item.id}
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
      ))}
    </div>
  )
}
