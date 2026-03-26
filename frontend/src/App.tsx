import { useState, useEffect } from 'react'
import NavSidebar from './components/NavSidebar'
import PaneContainer from './components/PaneContainer'
import ChatPane from './components/ChatPane'
import CollectionPanel from './components/CollectionPanel'
import type { Artifact, PaneData } from './types'
import { fetchArtifacts, persistArtifact, enrichArtifact } from './lib/api'

let nextId = 1

function App() {
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
      <header className="h-12 border-b border-gray-200 flex items-center px-5 shrink-0">
        <span className="font-semibold tracking-tight">Aladin</span>
      </header>

      <main className="flex-1 flex min-h-0">
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
      </main>
    </div>
  )
}

export default App
