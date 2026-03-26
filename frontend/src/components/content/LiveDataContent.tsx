export default function LiveDataContent() {
  return (
    <div className="h-full flex items-center justify-center">
      <div className="text-center space-y-2">
        <div className="w-10 h-10 rounded-full border-2 border-dashed border-gray-300 mx-auto flex items-center justify-center">
          <svg className="w-4 h-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
              d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </div>
        <p className="text-sm text-gray-400">Live data feeds coming soon</p>
      </div>
    </div>
  )
}
