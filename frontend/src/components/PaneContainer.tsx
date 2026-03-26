import type { Artifact, PaneData } from '../types'
import Pane from './Pane'

interface Props {
  panes: PaneData[]
  externalFile: File | null
  onAddPane: () => void
  onRemovePane: (id: string) => void
  onExternalFileConsumed: () => void
  onAddArtifact: (artifact: Artifact) => void
  artifactToAdd: Artifact | null
  onArtifactNodeAdded: () => void
  onNodeSelect: (nodeId: string) => void
}

export default function PaneContainer({ panes, externalFile, onAddPane, onRemovePane, onExternalFileConsumed, onAddArtifact, artifactToAdd, onArtifactNodeAdded, onNodeSelect }: Props) {
  return (
    <div className="flex-1 flex min-h-0">
      {panes.map((pane, index) => (
        <div
          key={pane.id}
          className={`flex-1 flex min-w-0 ${index > 0 ? 'border-l border-gray-200' : ''}`}
        >
          <Pane
            pane={pane}
            canClose={panes.length > 1}
            onClose={() => onRemovePane(pane.id)}
            onSplit={onAddPane}
            externalFile={index === 0 ? externalFile : null}
            onExternalFileConsumed={onExternalFileConsumed}
            onAddArtifact={onAddArtifact}
            artifactToAdd={artifactToAdd}
            onArtifactNodeAdded={onArtifactNodeAdded}
            onNodeSelect={onNodeSelect}
          />
        </div>
      ))}
    </div>
  )
}
