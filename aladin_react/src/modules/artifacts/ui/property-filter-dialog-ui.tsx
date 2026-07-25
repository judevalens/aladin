import { FileText, Filter, Layout, Link2, Mic, Paperclip, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { usePropertyFilter } from "@/modules/artifacts/hooks/use-property-filter";
import { useAppStore } from "@/app/state/store";

const KIND_ICONS: Record<string, typeof FileText> = {
  note: FileText,
  link: Link2,
  voice: Mic,
  file: Paperclip,
  app: Layout,
};

/**
 * H1c — "Filter by property": a lightweight database view over the workspace. Pick a property key
 * (and optionally a value) and see every artifact carrying it, wherever it lives in the tree.
 * Results come from a reactive server query, so editing a property updates the list live.
 */
export function PropertyFilterDialogUI({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { facets, facetsLoading, key, value, select, clear, results } = usePropertyFilter();
  const openArtifact = useAppStore((s) => s.openArtifact);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(94vw,760px)] p-0">
        <DialogHeader>
          <DialogTitle>Filter by property</DialogTitle>
        </DialogHeader>
        <div className="flex h-[min(70vh,520px)]">
          {/* left: the property keys/values actually in use */}
          <div className="w-[240px] shrink-0 overflow-y-auto border-r border-line p-2">
            {facetsLoading ? (
              <p className="px-2 py-4 text-[13px] text-ink-4">Loading…</p>
            ) : facets.length === 0 ? (
              <p className="px-2 py-4 text-[13px] text-ink-4">
                No properties yet. Add one from an artifact's inspector.
              </p>
            ) : (
              facets.map((f) => (
                <div key={f.key} className="mb-2">
                  <button
                    type="button"
                    onClick={() => select(f.key)}
                    className={cn(
                      "flex w-full items-center gap-2 rounded-chip px-2 py-1.5 text-left text-[13px] transition-colors",
                      key === f.key && !value ? "bg-raise text-ink" : "text-ink-2 hover:bg-card",
                    )}
                  >
                    <Filter className="size-3 text-ink-4" strokeWidth={1.75} />
                    <span className="flex-1 truncate font-display font-semibold">{f.key}</span>
                    <span className="font-mono text-[10px] text-ink-4">any</span>
                  </button>
                  <div className="ml-4 mt-0.5 flex flex-wrap gap-1">
                    {f.values.map((v) => (
                      <button
                        key={v}
                        type="button"
                        onClick={() => select(f.key, v)}
                        className={cn(
                          "rounded-chip border px-1.5 py-0.5 text-[11px] transition-colors",
                          key === f.key && value === v
                            ? "border-amber-line bg-amber-soft text-amber"
                            : "border-line text-ink-3 hover:border-line-2 hover:text-ink",
                        )}
                      >
                        {v}
                      </button>
                    ))}
                  </div>
                </div>
              ))
            )}
          </div>

          {/* right: matching artifacts */}
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2 text-[12px]">
              {key ? (
                <>
                  <span className="text-ink-3">
                    <span className="font-semibold text-ink">{key}</span>
                    {value ? ` = ${value}` : " (any value)"}
                  </span>
                  <button
                    type="button"
                    onClick={clear}
                    aria-label="Clear filter"
                    className="grid size-5 place-items-center rounded text-ink-4 hover:text-ink"
                  >
                    <X className="size-3.5" strokeWidth={1.75} />
                  </button>
                  <span className="ml-auto font-mono text-[11px] text-ink-4">
                    {results === undefined ? "…" : `${results.length} match`}
                  </span>
                </>
              ) : (
                <span className="text-ink-4">Pick a property to filter by.</span>
              )}
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto p-2">
              {!key ? null : results === undefined ? (
                <p className="px-2 py-8 text-center text-[13px] text-ink-4">Searching…</p>
              ) : results.length === 0 ? (
                <p className="px-2 py-8 text-center text-[13px] text-ink-4">No artifacts match.</p>
              ) : (
                results.map((a) => {
                  const Icon = KIND_ICONS[a.kind] ?? FileText;
                  return (
                    <button
                      key={a.id}
                      type="button"
                      onClick={() => {
                        openArtifact(a.id);
                        onOpenChange(false);
                      }}
                      className="flex w-full items-center gap-2 rounded-chip px-2 py-2 text-left transition-colors hover:bg-card"
                    >
                      <Icon className="size-3.5 shrink-0 text-ink-4" strokeWidth={1.75} />
                      <span className="flex-1 truncate text-[13px] text-ink">{a.title}</span>
                    </button>
                  );
                })
              )}
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
