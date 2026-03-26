import { useEffect, useRef, useState } from 'react'

interface Props {
  onSave: (filename: string, label: string) => void
  onCancel: () => void
}

function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60).toString().padStart(2, '0')
  const s = (seconds % 60).toString().padStart(2, '0')
  return `${m}:${s}`
}

export default function AudioRecorder({ onSave, onCancel }: Props) {
  const [duration, setDuration] = useState(0)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [volume, setVolume] = useState(0)

  const recorderRef = useRef<MediaRecorder | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const animFrameRef = useRef<number>(0)

  useEffect(() => {
    let stream: MediaStream

    async function init() {
      try {
        stream = await navigator.mediaDevices.getUserMedia({ audio: true })

        // Volume analyser
        const audioCtx = new AudioContext()
        const source = audioCtx.createMediaStreamSource(stream)
        const analyser = audioCtx.createAnalyser()
        analyser.fftSize = 256
        source.connect(analyser)
        const data = new Uint8Array(analyser.frequencyBinCount)

        function tick() {
          animFrameRef.current = requestAnimationFrame(tick)
          analyser.getByteFrequencyData(data)
          const avg = data.reduce((a, b) => a + b, 0) / data.length
          setVolume(avg / 128) // 0–1 range (can exceed 1 briefly)
        }
        tick()

        // Recorder
        const recorder = new MediaRecorder(stream)
        recorderRef.current = recorder
        chunksRef.current = []
        recorder.ondataavailable = (e) => { if (e.data.size > 0) chunksRef.current.push(e.data) }
        recorder.start()
        timerRef.current = setInterval(() => setDuration((d) => d + 1), 1000)
      } catch {
        setError('Microphone access denied.')
      }
    }

    init()

    return () => {
      cancelAnimationFrame(animFrameRef.current)
      if (timerRef.current) clearInterval(timerRef.current)
      stream?.getTracks().forEach((t) => t.stop())
    }
  }, [])

  async function handleSave() {
    const recorder = recorderRef.current
    if (!recorder) return
    setUploading(true)
    cancelAnimationFrame(animFrameRef.current)
    if (timerRef.current) clearInterval(timerRef.current)

    recorder.onstop = async () => {
      try {
        const blob = new Blob(chunksRef.current, { type: 'audio/webm' })
        const form = new FormData()
        form.append('file', blob, 'recording.webm')
        const res = await fetch('/api/audio/upload', { method: 'POST', body: form })
        if (!res.ok) throw new Error()
        const { filename } = await res.json()
        onSave(filename, `Recording ${formatDuration(duration)}`)
      } catch {
        setError('Failed to save.')
        setUploading(false)
      }
    }

    recorder.stop()
  }

  function handleDelete() {
    cancelAnimationFrame(animFrameRef.current)
    if (timerRef.current) clearInterval(timerRef.current)
    recorderRef.current?.stop()
    onCancel()
  }

  // Map volume to scale: 1.0 at silence, up to 1.8 at loud
  const ringScale = 1 + Math.min(volume * 1.5, 0.8)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30">
      <div className="bg-white rounded-2xl shadow-xl w-64 flex flex-col items-center px-6 py-7 gap-6">

        {/* Reactive circle */}
        <div className="relative flex items-center justify-center w-20 h-20">
          {/* Outer reactive ring */}
          <span
            className="absolute rounded-full bg-gray-200 transition-transform duration-75"
            style={{
              width: 80,
              height: 80,
              transform: `scale(${ringScale})`,
              opacity: 0.3 + volume * 0.3,
            }}
          />
          {/* Mid ring */}
          <span
            className="absolute rounded-full bg-gray-300 transition-transform duration-100"
            style={{
              width: 56,
              height: 56,
              transform: `scale(${1 + Math.min(volume * 0.6, 0.3)})`,
            }}
          />
          {/* Core dot */}
          <span className="relative w-8 h-8 rounded-full bg-gray-900" />
        </div>

        {/* Duration */}
        <span className="font-mono text-3xl font-semibold text-gray-900 tabular-nums tracking-wide">
          {formatDuration(duration)}
        </span>

        {error ? (
          <p className="text-sm text-red-500 text-center">{error}</p>
        ) : (
          <div className="flex gap-2 w-full">
            <button
              onClick={handleDelete}
              disabled={uploading}
              className="flex-1 py-2 rounded-xl border border-gray-200 text-sm text-gray-600 hover:bg-gray-50 disabled:opacity-40 transition-colors"
            >
              Delete
            </button>
            <button
              onClick={handleSave}
              disabled={uploading}
              className="flex-1 py-2 rounded-xl bg-gray-900 text-white text-sm font-medium hover:bg-gray-700 disabled:opacity-40 transition-colors"
            >
              {uploading ? 'Saving…' : 'Save'}
            </button>
          </div>
        )}

      </div>
    </div>
  )
}
