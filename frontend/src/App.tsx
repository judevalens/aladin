import { useState, useEffect } from 'react'
import NavSidebar from './components/NavSidebar'
import PaneContainer from './components/PaneContainer'
import ChatPane from './components/ChatPane'
import CollectionPanel from './components/CollectionPanel'
import Dashboard from './components/Dashboard'
import type { Artifact, PaneData } from './types'
import { fetchArtifacts, persistArtifact, enrichArtifact } from './lib/api'

let nextId = 1

type AppView = 'dashboard' | 'workspace'

function App() {
  const [view, setView] = useState<AppView>('dashboard')
  const [panes, setPanes] = useState<PaneData[]>([
    { id: String(nextId++), type: 'graph' },
  ])
  const [externalFile, setExternalFile] = useState<File | null>(null)
  const [activePanel, setActivePanel] = useState<string | null>(null)
  const [artifacts, setArtifacts] = useState<Artifact[]>([])
  const [artifactToAdd, setArtifactToAdd] = useState<Artifact | null>(null)
  const [selectedArtifactId, setSelectedArtifactId] = useState<string | null>(null)

  useEffect(() => {
    fetchArtifacts().then(setArtifacts).catch(console.error)
  }, [])

  const addPane = () =>
    setPanes((prev) => [...prev, { id: String(nextId++), type: 'graph' }])

  const removePane = (id: string) =>
    setPanes((prev) => prev.filter((p) => p.id !== id))

  const togglePanel = (panel: string) => {
    setActivePanel((prev) => (prev === panel ? null : panel))
    setSelectedArtifactId(null)
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

  const handleAddToGraph = (artifact: Artifact) => {
    setArtifactToAdd(artifact)
  }

  const handleNodeSelect = (nodeId: string) => {
    setActivePanel('collection')
    setSelectedArtifactId(nodeId)
  }

  return (
    <div className="h-screen flex flex-col bg-white text-gray-900">
      <header className="h-12 border-b border-gray-200 flex items-center px-5 shrink-0 gap-4">
        <span className="font-semibold tracking-tight">Aladin</span>
        <div className="flex gap-1 ml-2">
          {(['dashboard', 'workspace'] as AppView[]).map((v) => (
            <button
              key={v}
              onClick={() => setView(v)}
              className={`px-3 py-1 rounded-md text-xs font-medium capitalize transition-colors ${
                view === v ? 'bg-gray-900 text-white' : 'text-gray-500 hover:text-gray-900 hover:bg-gray-100'
              }`}
            >
              {v}
            </button>
          ))}
        </div>
      </header>

      <main className="flex-1 flex min-h-0">
        {view === 'dashboard' ? (
          <Dashboard
            artifacts={artifacts}
            onAddArtifact={(type, label, content, sourceUrl) => {
              handleAddArtifact({ id: `artifact-${Date.now()}`, type, label, content, sourceUrl, createdAt: new Date() })
            }}
            onAddToGraph={(artifact) => { handleAddToGraph(artifact); setView('workspace'); }}
            onOpenWorkspace={() => setView('workspace')}
          />
        ) : (
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
    </div>
  )
}

export default App
