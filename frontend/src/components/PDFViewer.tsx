import { Document, Page, pdfjs } from 'react-pdf'
import 'react-pdf/dist/Page/AnnotationLayer.css'
import 'react-pdf/dist/Page/TextLayer.css'

pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  'pdfjs-dist/build/pdf.worker.min.mjs',
  import.meta.url,
).toString()

interface Props {
  file: File | string
  page: number
  scale: number
  onLoadSuccess: (data: { numPages: number }) => void
}

export default function PDFViewer({ file, page, scale, onLoadSuccess }: Props) {
  return (
    <div className="h-full overflow-auto flex justify-center bg-gray-50 p-4">
      <Document
        file={file}
        onLoadSuccess={onLoadSuccess}
        loading={
          <div className="pt-20 text-sm text-gray-400">Loading…</div>
        }
      >
        <Page
          pageNumber={page}
          scale={scale}
          className="shadow-sm"
          renderTextLayer
          renderAnnotationLayer
        />
      </Document>
    </div>
  )
}
