import { useEffect, useRef, useState } from "react";
import { Icon } from "@/components/ui/icon";
import { Plus, Search } from "lucide-react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { EntityHit } from "@/modules/graph/graph-pane-types";

/**
 * EntityPicker is the shared typeahead for tagging a page with an entity: search the
 * registry, pick a match, or create a new entity. Resolution happens by the user's choice,
 * so there's no LLM adjudication at attach time.
 */
export function EntityPicker({
  onPick,
  onClose,
}: {
  onPick: (entityId: string) => void;
  onClose: () => void;
}) {
  const { repos } = useAppComposition();
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<EntityHit[]>([]);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Debounced search.
  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setHits([]);
      setLoading(false);
      return;
    }
    let cancelled = false;
    setLoading(true);
    const handle = setTimeout(() => {
      repos.graphPane
        .searchEntities(q)
        .then((res) => {
          if (!cancelled) setHits(res);
        })
        .catch(() => {
          if (!cancelled) setHits([]);
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 180);
    return () => {
      cancelled = true;
      clearTimeout(handle);
    };
  }, [repos.graphPane, query]);

  const exactExists = hits.some((h) => h.name.toLowerCase() === query.trim().toLowerCase());

  async function createAndPick() {
    const name = query.trim();
    if (!name || busy) return;
    setBusy(true);
    try {
      const created = await repos.graphPane.createEntity(name, "unknown");
      onPick(created.id);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-card border border-line bg-raise p-2 shadow-panel">
      <div className="flex items-center gap-2 rounded-chip border border-line bg-field px-2 py-1.5">
        <Icon as={Search} size="inline" mark className="shrink-0 text-ink-4" />
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") onClose();
          }}
          placeholder="search or create an entity…"
          spellCheck={false}
          className="w-full bg-transparent text-small text-ink outline-none placeholder:text-ink-4"
        />
      </div>

      {query.trim() ? (
        <ul className="mt-2 flex max-h-56 flex-col gap-0.5 overflow-auto">
          {hits.map((h) => (
            <li key={h.id}>
              <button
                type="button"
                onClick={() => onPick(h.id)}
                className="flex w-full items-center gap-2 rounded-chip px-2 py-1.5 text-left transition-colors hover:bg-[rgb(var(--hover))]"
              >
                <span className="font-mono text-meta uppercase text-ink-4">{h.kind || "entity"}</span>
                <span className="truncate text-body text-ink">{h.name}</span>
                {h.scope === "tenant" ? (
                  <span className="ml-auto font-mono text-meta text-ink-4">yours</span>
                ) : null}
              </button>
            </li>
          ))}
          {!loading && !exactExists ? (
            <li>
              <button
                type="button"
                onClick={createAndPick}
                disabled={busy}
                className="flex w-full items-center gap-2 rounded-chip px-2 py-1.5 text-left transition-colors hover:bg-[rgb(var(--hover))] disabled:opacity-50"
              >
                <Icon as={Plus} size="inline" mark className="text-amber" />
                <span className="text-body text-ink-2">
                  Create <span className="text-ink">“{query.trim()}”</span>
                </span>
              </button>
            </li>
          ) : null}
          {loading ? <li className="px-2 py-1.5 text-small text-ink-4">Searching…</li> : null}
        </ul>
      ) : (
        <p className="mt-2 px-1 text-small text-ink-4">Type to search entities.</p>
      )}
    </div>
  );
}
