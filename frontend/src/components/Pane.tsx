import { useEffect, useRef, useState } from 'react'
import type { Artifact, PaneData, PaneType } from '../types'
import { uploadDocument } from '../lib/api'
import PaneSwitcher from './PaneSwitcher'
import GraphContent from './content/GraphContent'
import D3GraphCanvas from './content/D3GraphCanvas'
import DocumentsContent from './content/DocumentsContent'
import LiveDataContent from './content/LiveDataContent'

const ZOOM_STEP = 0.25
const ZOOM_MIN  = 0.5
const ZOOM_MAX  = 3

const TYPE_LABELS: Record<PaneType, string> = {
  graph: 'Graph',
  documents: 'Documents',
  'live-data': 'Live Data',
}

interface Props {
  pane: PaneData
  canClose: boolean
  onClose: () => void
  onSplit: () => void
  externalFile?: File | null
  onExternalFileConsumed?: () => void
  onAddArtifact: (artifact: Artifact) => void
  artifactToAdd: Artifact | null
  onArtifactNodeAdded: () => void
  onNodeSelect: (nodeId: string) => void
}

function Divider() {
  return <div className="w-px h-4 bg-gray-200 mx-1" />
}

export default function Pane({ pane, canClose, onClose, onSplit, externalFile, onExternalFileConsumed, onAddArtifact, artifactToAdd, onArtifactNodeAdded, onNodeSelect }: Props) {
  const [type, setType]           = useState<PaneType>(pane.type)
  const [showSwitcher, setShowSwitcher] = useState(false)
  const [graphMode, setGraphMode] = useState<'reactflow' | 'd3'>('reactflow')

  // documents state
  const [file, setFile]           = useState<File | string | null>(null)
  const [filename, setFilename]   = useState('')
  const [page, setPage]           = useState(1)
  const [numPages, setNumPages]   = useState(0)
  const [scale, setScale]         = useState(1)

  const rootRef = useRef<HTMLButtonElement>(null)

  // When ChatPane uploads a file, switch this pane to Documents and load it
  useEffect(() => {
    if (!externalFile) return
    setType('documents')
    setFile(externalFile)
    setFilename(externalFile.name)
    setPage(1)
    onExternalFileConsumed?.()
    uploadDocument(externalFile).catch(console.error)
  }, [externalFile]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleFileLoad = (f: File) => {
    setFile(f)
    setFilename(f.name)
    setPage(1)
    uploadDocument(f).catch(console.error)
  }

  const handleDocumentSelect = (id: string, fname: string) => {
    setFile(`/api/documents/${id}/file`)
    setFilename(fname)
    setPage(1)
  }

  const zoomIn    = () => setScale((s) => Math.min(ZOOM_MAX, +(s + ZOOM_STEP).toFixed(2)))
  const zoomOut   = () => setScale((s) => Math.max(ZOOM_MIN, +(s - ZOOM_STEP).toFixed(2)))
  const zoomReset = () => setScale(1)

  const handleTypeChange = (newType: PaneType) => {
    setType(newType)
    setFile(null)
    setFilename('')
    setShowSwitcher(false)
  }

  const hasPDF = type === 'documents' && file !== null

  return (
    <div className="flex-1 flex flex-col min-w-0">
      {/* Header */}
      <div className="h-10 border-b border-gray-200 flex items-center justify-between px-3 shrink-0">

        {/* Breadcrumb */}
        <div className="flex items-center gap-1 text-xs min-w-0">
          <button
            ref={rootRef}
            onClick={() => setShowSwitcher((v) => !v)}
            className="text-gray-400 hover:text-gray-700 transition-colors shrink-0"
          >
            root
          </button>

          {showSwitcher && (
            <PaneSwitcher
              current={type}
              anchorRef={rootRef}
              onSelect={handleTypeChange}
              onClose={() => setShowSwitcher(false)}
            />
          )}

          <Chevron />
          <span className="text-gray-500 shrink-0">{TYPE_LABELS[type]}</span>

          {hasPDF && (
            <>
              <Chevron />
              <button
                onClick={() => { setFile(null); setFilename('') }}
                title="Back to document list"
                className="text-gray-700 hover:text-gray-900 truncate max-w-[160px] transition-colors"
              >
                {filename}
              </button>
            </>
          )}
        </div>

        {/* Controls */}
        <div className="flex items-center gap-0.5 shrink-0">
          {hasPDF && (
            <>
              <button onClick={zoomOut} disabled={scale <= ZOOM_MIN} title="Zoom out"
                className="p-1.5 rounded hover:bg-gray-100 disabled:opacity-30 transition-colors">
                <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M17 11A6 6 0 115 11a6 6 0 0112 0zM8 11h6" />
                </svg>
              </button>
              <button onClick={zoomReset} title="Reset zoom"
                className="text-xs text-gray-500 hover:text-gray-800 tabular-nums w-9 text-center transition-colors">
                {Math.round(scale * 100)}%
              </button>
              <button onClick={zoomIn} disabled={scale >= ZOOM_MAX} title="Zoom in"
                className="p-1.5 rounded hover:bg-gray-100 disabled:opacity-30 transition-colors">
                <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-4.35-4.35M17 11A6 6 0 115 11a6 6 0 0112 0zM11 8v6M8 11h6" />
                </svg>
              </button>
              <Divider />
              <button onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page <= 1}
                className="p-1.5 rounded hover:bg-gray-100 disabled:opacity-30 transition-colors">
                <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                </svg>
              </button>
              <span className="text-xs text-gray-500 tabular-nums px-1">{page} / {numPages}</span>
              <button onClick={() => setPage((p) => Math.min(numPages, p + 1))} disabled={page >= numPages}
                className="p-1.5 rounded hover:bg-gray-100 disabled:opacity-30 transition-colors">
                <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
              </button>
              <Divider />
            </>
          )}

          {type === 'graph' && (
            <>
              <div className="flex items-center rounded-md border border-gray-200 overflow-hidden mr-1">
                <button
                  onClick={() => setGraphMode('reactflow')}
                  className={`px-2 py-1 text-xs transition-colors ${graphMode === 'reactflow' ? 'bg-gray-900 text-white' : 'text-gray-400 hover:text-gray-700'}`}
                >
                  Flow
                </button>
                <button
                  onClick={() => setGraphMode('d3')}
                  className={`px-2 py-1 text-xs transition-colors ${graphMode === 'd3' ? 'bg-gray-900 text-white' : 'text-gray-400 hover:text-gray-700'}`}
                >
                  KG
                </button>
              </div>
              <Divider />
            </>
          )}

          {/* Split */}
          <button onClick={onSplit} title="Split pane"
            className="p-1.5 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors">
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
                d="M9 3H5a2 2 0 00-2 2v4m6-6h10a2 2 0 012 2v4M9 3v18m0 0h10a2 2 0 002-2V9M9 21H5a2 2 0 01-2-2V9m0 0h18" />
            </svg>
          </button>

          {/* Close pane */}
          {canClose && (
            <button onClick={onClose} title="Close pane"
              className="p-1.5 rounded hover:bg-gray-100 text-gray-400 hover:text-gray-600 transition-colors">
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-hidden">
        {type === 'graph' && graphMode === 'reactflow' && (
          <GraphContent
            onAddArtifact={onAddArtifact}
            artifactToAdd={artifactToAdd}
            onArtifactNodeAdded={onArtifactNodeAdded}
            onNodeSelect={onNodeSelect}
          />
        )}
        {type === 'graph' && graphMode === 'd3' && (
          <D3GraphCanvas
            onAddArtifact={onAddArtifact}
            artifactToAdd={artifactToAdd}
            onArtifactNodeAdded={onArtifactNodeAdded}
            onNodeSelect={onNodeSelect}
          />
        )}
        {type === 'documents' && (
          <DocumentsContent
            file={file}
            filename={filename}
            page={page}
            scale={scale}
            onFileLoad={handleFileLoad}
            onDocumentSelect={handleDocumentSelect}
            onLoadSuccess={({ numPages }) => { setNumPages(numPages); setPage(1) }}
          />
        )}
        {type === 'live-data' && <LiveDataContent />}
      </div>
    </div>
  )
}

function Chevron() {
  return (
    <svg className="w-3 h-3 text-gray-300 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
    </svg>
  )
}
