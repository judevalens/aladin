import { Plus, Tag, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";

import { useAppComposition } from "@/app/composition/app-composition";
import type { AttachedEntity, EntityHit } from "@/modules/graph/graph-pane-types";

// Y0.3 — the page-level entity tag bar: shows the entities a page is linked to (manual
// `tag`s, removable, plus read-only projected `@mention`s), and an "+ add" affordance that
// searches/creates an entity and attaches it. Grounds authored claim extraction (P3).
export function EntityTagBar({ pageId }: { pageId: string }) {
  const { repos } = useAppComposition();
  const [entities, setEntities] = useState<AttachedEntity[]>([]);
  const [adding, setAdding] = useState(false);

  const refresh = useCallback(() => {
    void repos.graphPane
      .listEntities(pageId)
      .then(setEntities)
      .catch(() => undefined);
  }, [repos, pageId]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  async function detach(entityId: string) {
    await repos.graphPane.detachEntity(pageId, entityId).catch(() => undefined);
    refresh();
  }

  async function attach(entityId: string) {
    await repos.graphPane.attachEntity(pageId, entityId).catch(() => undefined);
    setAdding(false);
    refresh();
  }

  const tags = entities.filter((e) => e.origin === "tag");
  // A mention already tagged shouldn't render twice; show only mention-origin entities not
  // also tagged.
  const tagIds = new Set(tags.map((t) => t.id));
  const mentions = entities.filter((e) => e.origin === "mention" && !tagIds.has(e.id));

  return (
    <div className="flex flex-wrap items-center gap-1.5 px-8 pb-1 pt-2">
      <Tag className="h-3.5 w-3.5 text-ink-4" />
      {tags.map((e) => (
        <span
          key={`tag-${e.id}`}
          className="group flex items-center gap-1 rounded-chip bg-amber-soft px-1.5 py-0.5 text-xs font-medium text-amber"
          title={e.kind}
        >
          {e.name}
          <button
            onClick={() => detach(e.id)}
            title="Remove tag"
            className="text-amber/60 hover:text-amber"
          >
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
      {mentions.map((e) => (
        <span
          key={`men-${e.id}`}
          className="rounded-chip bg-raise px-1.5 py-0.5 text-xs text-ink-3"
          title={`@mention · ${e.kind}`}
        >
          @{e.name}
        </span>
      ))}
      {adding ? (
        <EntityPicker
          onPick={attach}
          onClose={() => setAdding(false)}
          searchEntities={(q) => repos.graphPane.searchEntities(q)}
          createEntity={(name) => repos.graphPane.createEntity(name, "unknown")}
        />
      ) : (
        <button
          onClick={() => setAdding(true)}
          className="flex items-center gap-0.5 rounded-chip px-1.5 py-0.5 text-xs text-ink-4 hover:bg-raise hover:text-ink-2"
        >
          <Plus className="h-3 w-3" />
          Tag
        </button>
      )}
    </div>
  );
}

// A compact typeahead for attaching an entity: search existing, or create a new one.
function EntityPicker({
  onPick,
  onClose,
  searchEntities,
  createEntity,
}: {
  onPick: (entityId: string) => void;
  onClose: () => void;
  searchEntities: (q: string) => Promise<EntityHit[]>;
  createEntity: (name: string) => Promise<EntityHit>;
}) {
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<EntityHit[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    const q = query.trim();
    if (!q) {
      setHits([]);
      return;
    }
    let cancelled = false;
    void searchEntities(q).then((h) => {
      if (!cancelled) setHits(h);
    });
    return () => {
      cancelled = true;
    };
  }, [query, searchEntities]);

  const q = query.trim();
  const canCreate = q.length > 0 && !hits.some((h) => h.name.toLowerCase() === q.toLowerCase());

  async function create() {
    if (!q) return;
    const created = await createEntity(q);
    onPick(created.id);
  }

  return (
    <div className="relative">
      <input
        ref={inputRef}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") onClose();
          if (e.key === "Enter") {
            if (hits[0]) onPick(hits[0].id);
            else if (canCreate) void create();
          }
        }}
        onBlur={() => setTimeout(onClose, 150)}
        placeholder="Tag an entity…"
        className="w-40 rounded-chip border border-line bg-field px-2 py-0.5 text-xs text-ink outline-none focus:border-amber-line"
      />
      {q ? (
        <div className="absolute left-0 top-7 z-30 max-h-56 w-56 overflow-y-auto rounded-card border border-line bg-panel py-1 shadow-panel">
          {hits.map((h) => (
            <button
              key={h.id}
              onMouseDown={(e) => {
                e.preventDefault();
                onPick(h.id);
              }}
              className="flex w-full items-center justify-between px-2.5 py-1 text-left text-xs text-ink hover:bg-raise"
            >
              <span className="truncate">{h.name}</span>
              <span className="ml-2 shrink-0 text-ink-4">{h.kind}</span>
            </button>
          ))}
          {canCreate ? (
            <button
              onMouseDown={(e) => {
                e.preventDefault();
                void create();
              }}
              className="flex w-full items-center gap-1 px-2.5 py-1 text-left text-xs text-amber hover:bg-raise"
            >
              <Plus className="h-3 w-3" />
              Create “{q}”
            </button>
          ) : null}
          {hits.length === 0 && !canCreate ? (
            <div className="px-2.5 py-1 text-xs text-ink-4">No matches</div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
