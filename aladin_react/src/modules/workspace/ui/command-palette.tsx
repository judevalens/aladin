import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CandlestickChart, FilePlus2, FolderPlus, Link2, Mic, Upload } from "lucide-react";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { useAppComposition } from "@/app/composition/app-composition";
import type { InstrumentHit } from "@/shared/api/models";

export interface CommandPaletteActions {
  onCreateFolder: () => void;
  onCreateNote: () => void;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  onCreateFile: () => void;
}

/**
 * ⌘K command palette. Empty query → the create actions. Typing → ticker search over the
 * instruments registry (server-filtered, so cmdk's own filter is off). Selecting a ticker
 * routes to its detail surface. Ask-my-graph and richer navigation land later.
 */
export function CommandPalette({
  open,
  onOpenChange,
  actions,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  actions: CommandPaletteActions;
}) {
  const { repos } = useAppComposition();
  const navigate = useNavigate();
  const [query, setQuery] = useState("");
  const [tickers, setTickers] = useState<InstrumentHit[]>([]);
  const [searching, setSearching] = useState(false);

  // Reset the query each time the palette opens so it never reopens mid-search.
  useEffect(() => {
    if (open) setQuery("");
  }, [open]);

  // Debounced ticker search. Empty query clears results without a request.
  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setTickers([]);
      setSearching(false);
      return;
    }
    setSearching(true);
    let cancelled = false;
    const handle = setTimeout(() => {
      repos.instruments
        .search(q)
        .then((hits) => {
          if (!cancelled) setTickers(hits);
        })
        .catch(() => {
          if (!cancelled) setTickers([]);
        })
        .finally(() => {
          if (!cancelled) setSearching(false);
        });
    }, 180);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query, repos.instruments]);

  const run = (fn: () => void) => {
    onOpenChange(false);
    fn();
  };

  const openTicker = (symbol: string) => {
    onOpenChange(false);
    navigate(`/ticker/${encodeURIComponent(symbol)}`);
  };

  const hasQuery = query.trim().length > 0;

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} shouldFilter={false}>
      <CommandInput placeholder="Search tickers, or type a command…" value={query} onValueChange={setQuery} />
      <CommandList>
        {!hasQuery && (
          <CommandGroup heading="Create">
            <CommandItem value="new note" onSelect={() => run(actions.onCreateNote)}>
              <FilePlus2 className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              New note
            </CommandItem>
            <CommandItem value="new folder" onSelect={() => run(actions.onCreateFolder)}>
              <FolderPlus className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              New folder
            </CommandItem>
            <CommandItem value="new link" onSelect={() => run(actions.onCreateLink)}>
              <Link2 className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              New link
            </CommandItem>
            <CommandItem value="new voice note" onSelect={() => run(actions.onCreateVoice)}>
              <Mic className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              New voice note
            </CommandItem>
            <CommandItem value="upload file" onSelect={() => run(actions.onCreateFile)}>
              <Upload className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
              Upload file
            </CommandItem>
          </CommandGroup>
        )}

        {hasQuery && tickers.length > 0 && (
          <CommandGroup heading="Tickers">
            {tickers.map((t) => (
              <CommandItem key={t.id} value={t.id} onSelect={() => openTicker(t.symbol)}>
                <CandlestickChart className="h-[15px] w-[15px] text-ink-3" strokeWidth={1.75} />
                <span className="font-mono text-ink">{t.symbol}</span>
                <span className="truncate text-ink-3">{t.name}</span>
                <span className="ml-auto shrink-0 font-mono text-[10px] text-ink-4">{t.exchange}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        )}

        {hasQuery && !searching && tickers.length === 0 && <CommandEmpty>No tickers found.</CommandEmpty>}
      </CommandList>
    </CommandDialog>
  );
}
