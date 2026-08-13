import { useState } from "react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { FileText, Plus, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useGraphPane } from "@/modules/graph/hooks/use-graph-pane";
import { EntityPicker } from "@/modules/graph/ui/entity-picker";
import { EntityPeek } from "@/modules/entities/ui/entity-peek-ui";
import { PropertiesSection } from "@/modules/artifacts/ui/properties-editor-ui";
import type { Artifact } from "@/shared/api/models";
import type {
  GraphEntity,
  GraphLinkedArtifact,
  GraphPane,
} from "@/modules/graph/graph-pane-types";

// kindHue maps an entity kind onto the Aladin semantic ink ramp so the chips
// read at a glance without inventing new colors.
function kindHue(kind: string): string {
  switch (kind.toLowerCase()) {
    case "org":
    case "organization":
      return "text-amber";
    case "person":
      return "text-echo";
    case "product":
      return "text-catalyst";
    default:
      return "text-ink-3";
  }
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <Eyebrow as="h3" className="mb-2 text-ink-4">
      {children}
    </Eyebrow>
  );
}

function EntityList({
  artifactId,
  entities,
  onChanged,
  onOpenEntity,
}: {
  artifactId: string;
  entities: GraphEntity[];
  onChanged: () => void;
  onOpenEntity: (entityId: string) => void;
}) {
  const { repos } = useAppComposition();
  const [picking, setPicking] = useState(false);
  const [pendingId, setPendingId] = useState<string | null>(null);

  // NOTE on onChanged(): it is a LOCAL LATENCY SHORTCUT, not the sync mechanism. Correctness comes
  // from the syncer — the server emits a node frame for the artifact on attach/detach, and
  // useGraphPane refetches off that DataEvent, which is what makes OTHER windows (and async writes)
  // converge. We also refresh immediately here so the acting user isn't waiting on the pane's 500ms
  // debounce (that debounce exists to coalesce the editor's per-keystroke mention re-sync, which a
  // discrete click doesn't need). Do not treat this call as the update path.
  async function attach(entityId: string) {
    setPicking(false);
    setPendingId(entityId);
    try {
      await repos.graphPane.attachEntity(artifactId, entityId);
      onChanged();
    } finally {
      setPendingId(null);
    }
  }

  async function detach(entityId: string) {
    setPendingId(entityId);
    try {
      await repos.graphPane.detachEntity(artifactId, entityId);
      onChanged();
    } finally {
      setPendingId(null);
    }
  }

  return (
    <section>
      <div className="mb-2 flex items-center justify-between">
        <SectionLabel>Entities · {entities.length}</SectionLabel>
        <button
          type="button"
          aria-label="Add entity"
          onClick={() => setPicking((open) => !open)}
          className={cn(
            "flex size-5 items-center justify-center rounded text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink",
            picking && "bg-[rgb(var(--sel))] text-ink",
          )}
        >
          <Icon as={Plus} size="inline" mark />
        </button>
      </div>

      {picking ? (
        <div className="mb-2">
          <EntityPicker onPick={attach} onClose={() => setPicking(false)} />
        </div>
      ) : null}

      {entities.length === 0 ? (
        <p className="text-body text-ink-4">No entities on this page yet. Tag one with +.</p>
      ) : (
        <ul className="flex flex-wrap gap-2">
          {entities.map((e) => {
            const removable = e.origin === "tag";
            return (
              <li
                key={e.id}
                className={cn(
                  "flex items-center rounded-chip border bg-raise",
                  e.origin === "tag" ? "border-amber-line" : "border-line",
                  pendingId === e.id && "opacity-50",
                )}
              >
                <button
                  type="button"
                  aria-label={`Open ${e.name}`}
                  onClick={() => onOpenEntity(e.id)}
                  className="flex items-center gap-2 rounded-chip px-2.5 py-1 text-left transition-colors hover:bg-[rgb(var(--hover))]"
                >
                  <span className={cn("font-mono text-meta uppercase", kindHue(e.kind))}>
                    {e.kind || "entity"}
                  </span>
                  <span className="text-body text-ink">{e.name}</span>
                  {e.mentions > 0 ? (
                    <span className="font-mono text-meta text-ink-4">{e.mentions}×</span>
                  ) : null}
                </button>
                {removable ? (
                  <button
                    type="button"
                    aria-label={`Remove ${e.name}`}
                    onClick={() => detach(e.id)}
                    disabled={pendingId === e.id}
                    className="mr-1.5 flex size-4 items-center justify-center rounded text-ink-4 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
                  >
                    <Icon as={X} size="inline" mark />
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function linkedRelationLabel(relation: GraphLinkedArtifact["relation"]): string {
  switch (relation) {
    case "referenced_by":
      return "Links here";
    case "references":
      return "Referenced";
    case "shared_entity":
      return "Shared entity";
  }
}

function LinkedArtifactRow({ item }: { item: GraphLinkedArtifact }) {
  const openArtifact = useAppStore((state) => state.openArtifact);
  return (
    <li>
      <button
        type="button"
        onClick={() => openArtifact(item.id)}
        className="flex w-full items-center gap-2 rounded-card border border-line bg-card px-3 py-2 text-left transition-colors hover:bg-raise"
      >
        <Icon as={FileText} mark className="shrink-0 text-ink-4" />
        <span className="min-w-0 flex-1 truncate text-body text-ink-2">{item.title || item.id}</span>
        <span className="shrink-0 rounded-chip border border-line px-1.5 py-0.5 font-mono text-meta text-ink-4">
          {linkedRelationLabel(item.relation)}
        </span>
      </button>
    </li>
  );
}

function LinkedArtifactList({ items }: { items: GraphLinkedArtifact[] }) {
  return (
    <section>
      <SectionLabel>Linked artifacts · {items.length}</SectionLabel>
      {items.length === 0 ? (
        <p className="text-body text-ink-4">No other artifacts connected yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {items.map((item) => (
            <LinkedArtifactRow key={`${item.relation}-${item.id}`} item={item} />
          ))}
        </ul>
      )}
    </section>
  );
}

function PaneBody({
  artifactId,
  pane,
  onChanged,
  onOpenEntity,
}: {
  artifactId: string;
  pane: GraphPane;
  onChanged: () => void;
  onOpenEntity: (entityId: string) => void;
}) {
  return (
    <div className="flex flex-col gap-5">
      <EntityList
        artifactId={artifactId}
        entities={pane.entities}
        onChanged={onChanged}
        onOpenEntity={onOpenEntity}
      />
      <LinkedArtifactList items={pane.linkedArtifacts} />
    </div>
  );
}

/**
 * GraphSidePaneUI is the "On the graph" side pane docked beside the artifact you
 * are viewing. It keys off the active artifact and shows its place in the
 * knowledge graph — entities, connected claims, and cited sources.
 */
export function GraphSidePaneUI({
  artifact,
  onClose,
}: {
  artifact: Artifact;
  onClose: () => void;
}) {
  const artifactId = artifact.id;
  const { pane, loading, error, reload } = useGraphPane(artifactId);
  const [peekEntityId, setPeekEntityId] = useState<string | null>(null);

  return (
    <aside className="flex h-full w-[340px] shrink-0 flex-col overflow-hidden border-l border-line bg-panel">
      <header className="flex items-center gap-2 border-b border-line px-3 py-2.5">
        <span className="font-display text-body text-ink">On the graph</span>
        <button
          type="button"
          aria-label="Close graph pane"
          onClick={onClose}
          className="ml-auto flex size-6 items-center justify-center rounded text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
        >
          <Icon as={X} mark />
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-auto px-3 py-4">
        <div className="flex flex-col gap-5">
          {/* Properties come from the artifact itself, so they render immediately —
              independent of the graph pane's load state. */}
          <PropertiesSection artifact={artifact} />
          {/* Once we have a pane, keep showing it during a background refetch — otherwise
              a reactive refresh (every mention/ref sync) flashes the "Loading…" state. */}
          {pane ? (
            <PaneBody
              artifactId={artifactId}
              pane={pane}
              onChanged={reload}
              onOpenEntity={setPeekEntityId}
            />
          ) : loading ? (
            <p className="text-body text-ink-4">Loading…</p>
          ) : error ? (
            <p className="text-body text-against">{error}</p>
          ) : null}
        </div>
      </div>
      <EntityPeek entityId={peekEntityId} onClose={() => setPeekEntityId(null)} />
    </aside>
  );
}
