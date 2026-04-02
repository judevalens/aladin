import { useState, useEffect } from 'react'
import NavSidebar from './components/NavSidebar'
import PaneContainer from './components/PaneContainer'
import ChatPane from './components/ChatPane'
import CollectionPanel from './components/CollectionPanel'
import Dashboard from './components/Dashboard'
import UnifiedFeed from './components/UnifiedFeed'
import NoteComposer from './components/NoteComposer'
import InsightsPanel from './components/InsightsPanel'
import SearchPanel from './components/SearchPanel'
import GraphExplorer from './components/GraphExplorer'
import type { Artifact, PaneData, FeedItem, SearchResult } from './types'
import { fetchArtifacts, persistArtifact, enrichArtifact } from './lib/api'

let nextId = 1

type AppView = 'dashboard' | 'feed' | 'workspace'

function App() {
  const [view,               setView]               = useState<AppView>('dashboard')
  const [panes,              setPanes]              = useState<PaneData[]>([
    { id: String(nextId++), type: 'graph' },
  ])
  const [externalFile,       setExternalFile]       = useState<File | null>(null)
  const [activePanel,        setActivePanel]        = useState<string | null>(null)
  const [artifacts,          setArtifacts]          = useState<Artifact[]>([])
  const [artifactToAdd,      setArtifactToAdd]      = useState<Artifact | null>(null)
  const [selectedArtifactId, setSelectedArtifactId] = useState<string | null>(null)
  const [showNoteComposer,   setShowNoteComposer]   = useState(false)
  const [focusedFeedItem,    setFocusedFeedItem]    = useState<FeedItem | null>(null)

  useEffect(() => {
    fetchArtifacts().then(setArtifacts).catch(console.error)
  }, [])

  const addPane = () =>
    setPanes((prev) => [...prev, { id: String(nextId++), type: 'graph' }])

  const removePane = (id: string) =>
    setPanes((prev) => prev.filter((p) => p.id !== id))

  const togglePanel = (panel: string) => {
    setActivePanel((prev) => (prev === panel ? null : panel))
    if (panel !== 'collection') setSelectedArtifactId(null)
  }

  const handleAddArtifact = (artifact: Artifact) => {
    setArtifacts((prev) => [artifact, ...prev])
    setActivePanel('collection')
    setSelectedArtifactId(artifact.id)
    persistArtifact(artifact)
      .then(() => enrichArtifact(artifact.id))
      .then((enrichment) =>
        setArtifacts((prev) =>
          prev.map((a) => (a.id === artifact.id ? { ...a, enrichment } : a))
        )
      )
      .catch(console.error)
  }

  const handleNoteSaved = (note: Artifact) => {
    setArtifacts((prev) => [note, ...prev])
    setShowNoteComposer(false)
  }

  const handleAddToGraph = (artifact: Artifact) => {
    setArtifactToAdd(artifact)
  }

  const handleNodeSelect = (nodeId: string) => {
    setActivePanel('collection')
    setSelectedArtifactId(nodeId)
  }

  const handleOpenFeedArtifact = (item: FeedItem) => {
    setFocusedFeedItem(item)
    // Also add to artifact collection if not present
    if (!artifacts.find(a => a.id === item.id)) {
      setArtifacts(prev => [item, ...prev])
    }
  }

  const handleOpenSearchResult = (result: SearchResult) => {
    setSelectedArtifactId(result.id)
    setActivePanel('collection')
    setView('workspace')
    if (!artifacts.find(a => a.id === result.id)) {
      setArtifacts(prev => [result, ...prev])
    }
  }

  // Feed item focused → show as focus view in dashboard
  useEffect(() => {
    if (focusedFeedItem && view !== 'dashboard') {
      setView('dashboard')
    }
  }, [focusedFeedItem])

  const viewTabs: { id: AppView; label: string }[] = [
    { id: 'dashboard', label: 'Research' },
    { id: 'feed',      label: 'Feed' },
    { id: 'workspace', label: 'Workspace' },
  ]

  return (
    <div className="h-screen flex flex-col bg-white text-gray-900">
      <header className="h-12 border-b border-gray-200 flex items-center px-5 shrink-0 gap-4">
        <span className="font-semibold tracking-tight">Aladin</span>
        <div className="flex gap-1 ml-2">
          {viewTabs.map((v) => (
            <button
              key={v.id}
              onClick={() => setView(v.id)}
              className={`px-3 py-1 rounded-md text-xs font-medium capitalize transition-colors ${
                view === v.id ? 'bg-gray-900 text-white' : 'text-gray-500 hover:text-gray-900 hover:bg-gray-100'
              }`}
            >
              {v.label}
            </button>
          ))}
        </div>

        {/* Note composer trigger */}
        <button
          onClick={() => setShowNoteComposer(true)}
          className="ml-auto flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-gray-200 text-xs text-gray-500 hover:bg-gray-50 transition-colors"
        >
          + Note
        </button>
      </header>

      <main className="flex-1 flex min-h-0">
        {view === 'dashboard' && (
          <Dashboard
            artifacts={artifacts}
            focusedArtifact={focusedFeedItem ?? undefined}
            onClearFocus={() => setFocusedFeedItem(null)}
            onAddArtifact={(type, label, content, sourceUrl) => {
              handleAddArtifact({
                id: `artifact-${Date.now()}`,
                type, label, content, sourceUrl,
                createdAt: new Date(),
              })
            }}
            onAddToGraph={(artifact) => {
              handleAddToGraph(artifact)
              setView('workspace')
            }}
            onOpenWorkspace={() => setView('workspace')}
          />
        )}

        {view === 'feed' && (
          <UnifiedFeed onOpenArtifact={handleOpenFeedArtifact} />
        )}

        {view === 'workspace' && (
          <>
            <NavSidebar activePanel={activePanel} onTogglePanel={togglePanel} />

            {activePanel === 'collection' && (
              <CollectionPanel
                artifacts={artifacts}
                selectedArtifactId={selectedArtifactId}
                onSelectArtifact={setSelectedArtifactId}
                onAddToGraph={handleAddToGraph}
                onDeleteArtifact={(id) => setArtifacts((prev) => prev.filter((a) => a.id !== id))}
              />
            )}

            {activePanel === 'search' && (
              <div className="w-80 shrink-0 border-r border-gray-100 flex flex-col">
                <SearchPanel onOpenArtifact={handleOpenSearchResult} />
              </div>
            )}

            {activePanel === 'insights' && (
              <div className="w-80 shrink-0 border-r border-gray-100 flex flex-col overflow-hidden">
                <InsightsPanel
                  onViewArtifacts={(ids) => {
                    const firstId = ids[0]
                    if (firstId) {
                      setSelectedArtifactId(firstId)
                      setActivePanel('collection')
                    }
                  }}
                />
              </div>
            )}

            {activePanel === 'graph-explorer' && (
              <div className="w-72 shrink-0 border-r border-gray-100 flex flex-col overflow-hidden">
                <GraphExplorer
                  onSelectArtifact={(id) => {
                    setSelectedArtifactId(id)
                    setActivePanel('collection')
                  }}
                />
              </div>
            )}

            <PaneContainer
              panes={panes}
              externalFile={externalFile}
              onAddPane={addPane}
              onRemovePane={removePane}
              onExternalFileConsumed={() => setExternalFile(null)}
              onAddArtifact={handleAddArtifact}
              artifactToAdd={artifactToAdd}
              onArtifactNodeAdded={() => setArtifactToAdd(null)}
              onNodeSelect={handleNodeSelect}
            />
            <ChatPane onFileUpload={setExternalFile} />
          </>
        )}
      </main>

      {/* Note composer overlay */}
      {showNoteComposer && (
        <NoteComposer
          onSave={handleNoteSaved}
          onCancel={() => setShowNoteComposer(false)}
        />
      )}
    </div>
  )
}

export default App
