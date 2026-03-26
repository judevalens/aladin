import { useEffect, useRef, useState } from 'react'
import PDFViewer from '../PDFViewer'

interface DocMeta {
  id: string
  filename: string
  title: string | null
  page_count: number | null
  uploaded_at: string
}

interface Props {
  file: File | string | null
  filename: string  // used by Pane breadcrumb, not here
  page: number
  scale: number
  onFileLoad: (file: File) => void
  onDocumentSelect: (id: string, filename: string) => void
  onLoadSuccess: (data: { numPages: number }) => void
}

export default function DocumentsContent({
  file, page, scale, onFileLoad, onDocumentSelect, onLoadSuccess,
}: Props) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [docs, setDocs] = useState<DocMeta[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/documents/')
      .then((r) => r.json())
      .then(setDocs)
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [file]) // refetch after a new upload opens

  if (file) {
    return (
      <PDFViewer
        file={file}
        page={page}
        scale={scale}
        onLoadSuccess={onLoadSuccess}
      />
    )
  }

  return (
    <div className="h-full flex flex-col">
      {/* Upload */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100">
        <input
          ref={inputRef}
          type="file"
          accept=".pdf"
          className="hidden"
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) onFileLoad(f)
            e.target.value = ''
          }}
        />
        <button
          onClick={() => inputRef.current?.click()}
          className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-lg border border-dashed border-gray-300 text-sm text-gray-500 hover:border-gray-400 hover:text-gray-700 transition-colors"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75}
              d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
          </svg>
          Upload PDF
        </button>
      </div>

      {/* Document list */}
      <div className="flex-1 overflow-y-auto">
        {loading && (
          <p className="text-xs text-gray-400 text-center mt-8">Loading…</p>
        )}
        {!loading && docs.length === 0 && (
          <p className="text-xs text-gray-400 text-center mt-8">No documents yet</p>
        )}
        {docs.map((doc) => (
          <button
            key={doc.id}
            onClick={() => onDocumentSelect(doc.id, doc.filename)}
            className="w-full text-left px-4 py-3 border-b border-gray-100 hover:bg-gray-50 transition-colors group"
          >
            <p className="text-sm text-gray-800 truncate">
              {doc.title || doc.filename}
            </p>
            <div className="flex items-center gap-2 mt-0.5">
              {doc.page_count && (
                <span className="text-xs text-gray-400">{doc.page_count}p</span>
              )}
              <span className="text-xs text-gray-400">
                {new Date(doc.uploaded_at).toLocaleDateString()}
              </span>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}
