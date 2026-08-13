import { useEffect, useState } from "react";
import { Icon } from "@/components/ui/icon";
import { useNavigate } from "react-router-dom";
import { Building2, CandlestickChart, FilePlus2, FileText, FlaskConical, FolderPlus, Globe, Layers, Link2, Mic, Upload, User } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import type { SearchHit, SearchSection } from "@/shared/api/models";

export interface CommandPaletteActions {
  onCreateFolder: () => void;
  onCreateResearch: () => void;
  onCreateNote: () => void;
  onCreateLink: () => void;
  onCreateVoice: () => void;
  onCreateFile: () => void;
}

// Presentation-only: hit.kind → row icon. Routing/icons are legitimately client concerns;
// the search logic (federation, sections, order) is owned by the backend.
const KIND_ICON: Record<string, LucideIcon> = {
  ticker: CandlestickChart,
  company: Building2,
  person: User,
  entity: Globe,
  page: FileText,
  shard: Layers,
};

/**
 * ⌘K command palette — a GLOBAL search. Empty query → the create actions. Typing calls the
 * one federated /api/search endpoint and renders whatever sections it returns, generically.
 * The backend owns which sources exist, how they're grouped, and their order; this component
 * just maps each hit's `kind` to an icon and a route.
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
  const openArtifact = useAppStore((state) => state.openArtifact);
  const openTicker = useAppStore((state) => state.openTicker);
  const [query, setQuery] = useState("");
  const [sections, setSections] = useState<SearchSection[]>([]);
  const [searching, setSearching] = useState(false);

  // Reset the query each time the palette opens so it never reopens mid-search.
  useEffect(() => {
    if (open) setQuery("");
  }, [open]);

  // Debounced global search — one call, backend-federated.
  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setSections([]);
      setSearching(false);
      return;
    }
    setSearching(true);
    let cancelled = false;
    const handle = setTimeout(() => {
      repos.search
        .query(q)
        .then((res) => {
          if (!cancelled) setSections(res.sections);
        })
        .catch(() => {
          if (!cancelled) setSections([]);
        })
        .finally(() => {
          if (!cancelled) setSearching(false);
        });
    }, 180);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [query, repos.search]);

  const run = (fn: () => void) => {
    onOpenChange(false);
    fn();
  };

  // hit.kind → navigation. Tickers open the ticker detail; entity kinds open the entity
  // surface; pages/shards open the artifact in the workspace.
  const openHit = (hit: SearchHit) => {
    onOpenChange(false);
    if (hit.kind === "ticker") {
      openTicker(hit.title);
    } else if (hit.kind === "page" || hit.kind === "shard") {
      openArtifact(hit.id);
      navigate("/folders");
    } else {
      navigate(`/entity/${hit.id}`);
    }
  };

  const hasQuery = query.trim().length > 0;
  const empty = hasQuery && !searching && sections.length === 0;

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} shouldFilter={false}>
      <CommandInput
        placeholder="Search tickers, companies, people, pages… or type a command"
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {!hasQuery && (
          <CommandGroup heading="Create">
            <CommandItem value="new note" onSelect={() => run(actions.onCreateNote)}>
              <Icon as={FilePlus2} className="text-ink-3" />
              New note
            </CommandItem>
            <CommandItem value="new folder" onSelect={() => run(actions.onCreateFolder)}>
              <Icon as={FolderPlus} className="text-ink-3" />
              New folder
            </CommandItem>
            <CommandItem value="new research" onSelect={() => run(actions.onCreateResearch)}>
              <Icon as={FlaskConical} className="text-ink-3" />
              New research
            </CommandItem>
            <CommandItem value="new link" onSelect={() => run(actions.onCreateLink)}>
              <Icon as={Link2} className="text-ink-3" />
              New link
            </CommandItem>
            <CommandItem value="new voice note" onSelect={() => run(actions.onCreateVoice)}>
              <Icon as={Mic} className="text-ink-3" />
              New voice note
            </CommandItem>
            <CommandItem value="upload file" onSelect={() => run(actions.onCreateFile)}>
              <Icon as={Upload} className="text-ink-3" />
              Upload file
            </CommandItem>
          </CommandGroup>
        )}

        {sections.map((section) => (
          <CommandGroup key={section.type} heading={section.label}>
            {section.hits.map((hit) => {
              const Glyph = KIND_ICON[hit.kind] ?? Globe;
              return (
                <CommandItem key={`${hit.kind}-${hit.id}`} value={`${hit.kind}-${hit.id}`} onSelect={() => openHit(hit)}>
                  <Icon as={Glyph} className="text-ink-3" />
                  <span className={hit.kind === "ticker" ? "font-mono text-ink" : "text-ink"}>
                    {hit.title}
                  </span>
                  {hit.subtitle && <span className="truncate text-ink-3">{hit.subtitle}</span>}
                  <span className="ml-auto shrink-0 font-mono text-meta uppercase text-ink-4">
                    {hit.kind}
                  </span>
                </CommandItem>
              );
            })}
          </CommandGroup>
        ))}

        {empty && <CommandEmpty>No results.</CommandEmpty>}
      </CommandList>
    </CommandDialog>
  );
}
