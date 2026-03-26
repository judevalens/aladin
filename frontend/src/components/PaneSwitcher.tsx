import { useEffect, useRef, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import type { PaneType } from '../types'

interface Option {
  type: PaneType
  label: string
  description: string
}

const OPTIONS: Option[] = [
  { type: 'graph',     label: 'Graph',     description: 'Knowledge graph visualization' },
  { type: 'documents', label: 'Documents', description: 'View and annotate PDFs' },
  { type: 'live-data', label: 'Live Data', description: 'Real-time data feeds' },
]

interface Props {
  current: PaneType
  anchorRef: RefObject<HTMLButtonElement | null>
  onSelect: (type: PaneType) => void
  onClose: () => void
}

export default function PaneSwitcher({ current, anchorRef, onSelect, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const rect = anchorRef.current?.getBoundingClientRect()

  useEffect(() => {
    const handle = (e: MouseEvent) => {
      if (
        ref.current && !ref.current.contains(e.target as Node) &&
        anchorRef.current && !anchorRef.current.contains(e.target as Node)
      ) {
        onClose()
      }
    }
    document.addEventListener('mousedown', handle)
    return () => document.removeEventListener('mousedown', handle)
  }, [anchorRef, onClose])

  if (!rect) return null

  return createPortal(
    <div
      ref={ref}
      style={{ position: 'fixed', top: rect.bottom + 6, left: rect.left }}
      className="z-50 bg-white border border-gray-200 rounded-xl shadow-lg py-1 w-52"
    >
      {OPTIONS.map((opt) => (
        <button
          key={opt.type}
          onClick={() => onSelect(opt.type)}
          className="w-full text-left px-3 py-2 hover:bg-gray-50 transition-colors flex items-start justify-between gap-2 group"
        >
          <div>
            <p className="text-sm font-medium text-gray-800">{opt.label}</p>
            <p className="text-xs text-gray-400">{opt.description}</p>
          </div>
          {current === opt.type && (
            <svg className="w-3.5 h-3.5 text-gray-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2.5} d="M5 13l4 4L19 7" />
            </svg>
          )}
        </button>
      ))}
    </div>,
    document.body,
  )
}
