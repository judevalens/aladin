import { LoaderCircle, RotateCcw } from "lucide-react";

export function ShardAccessNotice({ error, retry }: { error: boolean; retry: () => void }) {
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-3 px-6 text-center text-small text-ink-3">
      {error ? (
        <>
          <p role="alert">Could not authenticate this shard.</p>
          <button type="button" onClick={retry} className="inline-flex items-center gap-2 rounded-control border border-line px-3 py-2 text-ink-2 hover:bg-panel">
            <RotateCcw className="size-4" aria-hidden="true" />
            Retry
          </button>
        </>
      ) : (
        <p role="status" className="inline-flex items-center gap-2">
          <LoaderCircle className="size-4 animate-spin" aria-hidden="true" />
          Opening shard...
        </p>
      )}
    </div>
  );
}
