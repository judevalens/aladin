import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Pencil, Plus, Search, Trash2, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { useAppComposition } from "@/app/composition/app-composition";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useWatchlists } from "@/modules/markets/hooks/use-watchlists";
import type { InstrumentHit } from "@/shared/api/models";

/**
 * The watchlist manager: open a list, see everything in it, add/remove symbols — the primary
 * curation surface. Left rail = all lists (select / new / rename / delete); right pane = the
 * selected list's members + a ticker search to add. Everything reads/writes the frame-fed store,
 * so edits here show up live everywhere (map, table, star menu).
 */
export function WatchlistManagerDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { repos } = useAppComposition();
  const { lists, activeId, setActive, create, rename, remove, addItem, removeItem } = useWatchlists();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [renaming, setRenaming] = useState<string | null>(null);
  const [renameText, setRenameText] = useState("");
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  // Ticker search (add box)
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<InstrumentHit[]>([]);
  const searchSeq = useRef(0);

  // Default the selected list to the active one when the modal opens.
  useEffect(() => {
    if (!open) return;
    setSelectedId((cur) => cur ?? activeId ?? lists[0]?.id ?? null);
  }, [open, activeId, lists]);

  // Keep the selection valid (a deleted list falls back to the first).
  useEffect(() => {
    if (selectedId && !lists.some((l) => l.id === selectedId)) {
      setSelectedId(lists[0]?.id ?? null);
    }
  }, [lists, selectedId]);

  const selected = useMemo(() => lists.find((l) => l.id === selectedId) ?? null, [lists, selectedId]);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 1) {
      setHits([]);
      return;
    }
    const seq = ++searchSeq.current;
    repos.instruments
      .search(q, 6)
      .then((r) => {
        if (seq === searchSeq.current) setHits(r);
      })
      .catch(() => {
        if (seq === searchSeq.current) setHits([]);
      });
  }, [query, repos]);

  async function createList() {
    const name = newName.trim();
    if (!name) return;
    const id = await create(name);
    setNewName("");
    if (id) setSelectedId(id);
  }

  const inSelected = (symbol: string) => selected?.items.some((i) => i.symbol === symbol) ?? false;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(94vw,760px)] p-0">
        <DialogHeader>
          <DialogTitle>Watchlists</DialogTitle>
        </DialogHeader>
        <div className="flex h-[min(70vh,520px)]">
          {/* left rail: all lists */}
          <div className="flex w-[240px] shrink-0 flex-col border-r border-line">
            <div className="min-h-0 flex-1 overflow-y-auto p-2">
              {lists.map((l) => (
                <div
                  key={l.id}
                  className={cn(
                    "group/row flex items-center gap-2 rounded-chip px-2 py-1.5 text-[13px] transition-colors",
                    l.id === selectedId ? "bg-raise text-ink" : "text-ink-2 hover:bg-card",
                  )}
                >
                  {renaming === l.id ? (
                    <Input
                      autoFocus
                      value={renameText}
                      onChange={(e) => setRenameText(e.target.value)}
                      onBlur={() => setRenaming(null)}
                      onKeyDown={async (e) => {
                        if (e.key === "Enter") {
                          const n = renameText.trim();
                          if (n && n !== l.name) await rename(l.id, n);
                          setRenaming(null);
                        } else if (e.key === "Escape") setRenaming(null);
                      }}
                      className="h-6 px-1.5 py-0 text-[13px]"
                    />
                  ) : confirmDelete === l.id ? (
                    <>
                      <span className="flex-1 truncate text-against">Delete “{l.name}”?</span>
                      <button
                        type="button"
                        aria-label="Confirm delete"
                        onClick={async () => {
                          setConfirmDelete(null);
                          await remove(l.id);
                        }}
                        className="grid size-5 place-items-center rounded text-against hover:bg-against/10"
                      >
                        <Check className="size-3.5" strokeWidth={2} />
                      </button>
                      <button
                        type="button"
                        aria-label="Cancel delete"
                        onClick={() => setConfirmDelete(null)}
                        className="grid size-5 place-items-center rounded text-ink-4 hover:text-ink"
                      >
                        <X className="size-3.5" strokeWidth={2} />
                      </button>
                    </>
                  ) : (
                    <>
                      <button
                        type="button"
                        onClick={() => setSelectedId(l.id)}
                        className="flex min-w-0 flex-1 items-center gap-2 text-left"
                      >
                        <span className="flex-1 truncate font-display font-semibold">{l.name}</span>
                        <span className="font-mono text-[11px] text-ink-4">{l.itemCount}</span>
                      </button>
                      <span className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover/row:opacity-100">
                        <button
                          type="button"
                          aria-label={`Rename ${l.name}`}
                          onClick={() => {
                            setRenameText(l.name);
                            setRenaming(l.id);
                          }}
                          className="grid size-5 place-items-center rounded text-ink-4 hover:text-ink"
                        >
                          <Pencil className="size-3" strokeWidth={1.75} />
                        </button>
                        <button
                          type="button"
                          aria-label={`Delete ${l.name}`}
                          onClick={() => setConfirmDelete(l.id)}
                          className="grid size-5 place-items-center rounded text-ink-4 hover:text-against"
                        >
                          <Trash2 className="size-3" strokeWidth={1.75} />
                        </button>
                      </span>
                    </>
                  )}
                </div>
              ))}
            </div>
            <div className="flex items-center gap-1.5 border-t border-line p-2">
              <Input
                placeholder="New list…"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void createList();
                }}
                className="h-7 text-[13px]"
              />
              <button
                type="button"
                aria-label="Create list"
                onClick={createList}
                disabled={!newName.trim()}
                className="grid size-7 shrink-0 place-items-center rounded-card bg-amber text-bg disabled:opacity-40"
              >
                <Plus className="size-4" strokeWidth={2.5} />
              </button>
            </div>
          </div>

          {/* right pane: selected list's members + add */}
          <div className="flex min-w-0 flex-1 flex-col">
            {selected ? (
              <>
                <div className="shrink-0 border-b border-line p-3">
                  <div className="relative">
                    <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-ink-4" strokeWidth={1.75} />
                    <Input
                      placeholder={`Add a ticker to ${selected.name}…`}
                      value={query}
                      onChange={(e) => setQuery(e.target.value)}
                      className="h-8 pl-8 text-[13px]"
                    />
                  </div>
                  {hits.length > 0 && (
                    <div className="mt-1.5 overflow-hidden rounded-card border border-line bg-panel">
                      {hits.map((h) => {
                        const added = inSelected(h.symbol);
                        return (
                          <button
                            key={h.id}
                            type="button"
                            disabled={added}
                            onClick={async () => {
                              await addItem(selected.id, h.id);
                              setQuery("");
                            }}
                            className="flex w-full items-center gap-2 px-2.5 py-1.5 text-left text-[13px] transition-colors hover:bg-raise disabled:opacity-50"
                          >
                            <span className="font-display font-semibold text-ink">{h.symbol}</span>
                            <span className="flex-1 truncate text-[12px] text-ink-4">{h.name}</span>
                            {added ? (
                              <span className="text-[11px] text-ink-4">added</span>
                            ) : (
                              <Plus className="size-3.5 text-amber" strokeWidth={2} />
                            )}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto p-2">
                  {selected.items.length === 0 ? (
                    <p className="px-2 py-8 text-center text-[13px] text-ink-4">
                      No tickers yet — search above to add.
                    </p>
                  ) : (
                    selected.items.map((it) => (
                      <div
                        key={it.instrumentId}
                        className="group/item flex items-center gap-2 rounded-chip px-2 py-2 hover:bg-card"
                      >
                        <span className="font-display text-[13px] font-semibold text-ink">{it.symbol}</span>
                        <span className="flex-1 truncate text-[12px] text-ink-4">{it.name}</span>
                        <button
                          type="button"
                          aria-label={`Remove ${it.symbol}`}
                          onClick={() => removeItem(selected.id, it.instrumentId)}
                          className="grid size-6 place-items-center rounded text-ink-4 opacity-0 transition-opacity hover:text-against group-hover/item:opacity-100"
                        >
                          <X className="size-4" strokeWidth={1.75} />
                        </button>
                      </div>
                    ))
                  )}
                </div>
                <div className="shrink-0 border-t border-line px-3 py-2">
                  <button
                    type="button"
                    onClick={() => {
                      setActive(selected.id);
                      onOpenChange(false);
                    }}
                    className="text-[12px] text-amber hover:underline"
                  >
                    Show “{selected.name}” on the map →
                  </button>
                </div>
              </>
            ) : (
              <div className="grid flex-1 place-items-center text-[13px] text-ink-4">
                Create a list to get started.
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
