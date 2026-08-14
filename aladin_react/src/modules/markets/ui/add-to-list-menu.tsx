import { useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { Check, ListPlus, Loader2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { useAppComposition } from "@/app/composition/app-composition";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useWatchlists } from "@/modules/markets/hooks/use-watchlists";

/**
 * The "add to list…" control: pick the stock first (the trigger), then toggle which lists it
 * belongs to — no pre-selecting an active list. Membership is read from the frame-fed store; an
 * add resolves the instrument id from a sibling list that already has it, else via ticker search.
 */
export function AddToListMenu({
  symbol,
  children,
  align = "end",
}: {
  symbol: string;
  children: React.ReactNode;
  align?: "start" | "center" | "end";
}) {
  const { repos } = useAppComposition();
  const { lists, addItem, removeItem, create } = useWatchlists();
  const [pending, setPending] = useState<string | null>(null); // listId being toggled

  // The instrument id for this symbol, if any list already holds it (avoids a search on remove).
  const known = lists.flatMap((l) => l.items).find((i) => i.symbol === symbol)?.instrumentId ?? null;

  async function toggle(listId: string) {
    const list = lists.find((l) => l.id === listId);
    if (!list) return;
    const existing = list.items.find((i) => i.symbol === symbol);
    setPending(listId);
    try {
      if (existing) {
        await removeItem(listId, existing.instrumentId);
        return;
      }
      let instrumentId = known;
      if (!instrumentId) {
        const hits = await repos.instruments.search(symbol, 5);
        instrumentId = hits.find((h) => h.symbol.toUpperCase() === symbol.toUpperCase())?.id ?? null;
      }
      if (instrumentId) await addItem(listId, instrumentId);
    } finally {
      setPending(null);
    }
  }

  async function addToNew() {
    const id = await create(`Watchlist`);
    if (id) await toggle(id);
  }

  return (
    <Popover>
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent align={align} className="w-56 p-1">
        <Eyebrow className="px-2 py-1.5 text-ink-4">
          Add {symbol} to…
        </Eyebrow>
        <div className="max-h-64 overflow-y-auto">
          {lists.length === 0 ? (
            <div className="px-2 py-2 text-small text-ink-4">No lists yet</div>
          ) : (
            lists.map((l) => {
              const inList = l.items.some((i) => i.symbol === symbol);
              return (
                <button
                  key={l.id}
                  type="button"
                  onClick={() => toggle(l.id)}
                  disabled={pending !== null}
                  className="flex w-full items-center gap-2 rounded-chip px-2 py-1.5 text-left text-body text-ink-2 transition-colors hover:bg-raise disabled:opacity-60"
                >
                  <span
                    className={cn(
                      "grid size-4 shrink-0 place-items-center rounded border",
                      inList ? "border-amber bg-amber/15" : "border-line-2",
                    )}
                  >
                    {pending === l.id ? (
                      <Icon as={Loader2} size="inline" mark className="animate-spin text-ink-4" />
                    ) : inList ? (
                      <Icon as={Check} size="inline" mark className="text-amber" />
                    ) : null}
                  </span>
                  <span className="flex-1 truncate">{l.name}</span>
                  <span className="font-mono text-meta text-ink-4">{l.itemCount}</span>
                </button>
              );
            })
          )}
        </div>
        <div className="mt-1 border-t border-line pt-1">
          <button
            type="button"
            onClick={addToNew}
            disabled={pending !== null}
            className="flex w-full items-center gap-2 rounded-chip px-2 py-1.5 text-left text-body text-ink-3 transition-colors hover:bg-raise hover:text-ink disabled:opacity-60"
          >
            <Icon as={ListPlus} size="inline" className="text-ink-4" />
            New list with {symbol}
          </button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
