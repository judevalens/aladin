import { useState } from "react";
import { Check, FileText, Link2, Plus, Quote, X, Zap } from "lucide-react";

import { cn } from "@/lib/utils";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";
import { useGraphPane } from "@/modules/graph/hooks/use-graph-pane";
import { EntityPicker } from "@/modules/graph/ui/entity-picker";
import { EntityPeek } from "@/modules/entities/ui/entity-peek-ui";
import { PropertiesSection } from "@/modules/artifacts/ui/properties-editor-ui";
import type { Artifact } from "@/shared/api/models";
import type {
  GraphCite,
  GraphClaim,
  GraphEntity,
  GraphLinkedArtifact,
  GraphPane,
  GraphThesis,
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
    <h3 className="mb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-4">
      {children}
    </h3>
  );
}

function ThesisCard({ thesis }: { thesis: GraphThesis }) {
  return (
    <section>
      <SectionLabel>Thesis</SectionLabel>
      <div className="rounded-card border border-amber-line bg-card p-3">
        <p className="font-display text-[14px] leading-snug text-ink">{thesis.text}</p>
        <div className="mt-2.5 flex flex-wrap items-center gap-2">
          <span className="rounded-chip bg-amber-soft px-2 py-0.5 font-mono text-[11px] text-amber">
            {thesis.polarity || "assert"}
          </span>
          {thesis.trustTier ? (
            <span className="rounded-chip border border-line px-2 py-0.5 font-mono text-[11px] text-ink-3">
              {thesis.trustTier}
            </span>
          ) : null}
        </div>
      </div>
    </section>
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
          <Plus className="size-3.5" />
        </button>
      </div>

      {picking ? (
        <div className="mb-2">
          <EntityPicker onPick={attach} onClose={() => setPicking(false)} />
        </div>
      ) : null}

      {entities.length === 0 ? (
        <p className="text-[13px] text-ink-4">No entities on this page yet. Tag one with +.</p>
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
                  <span className={cn("font-mono text-[10px] uppercase", kindHue(e.kind))}>
                    {e.kind || "entity"}
                  </span>
                  <span className="text-[13px] text-ink">{e.name}</span>
                  {e.mentions > 0 ? (
                    <span className="font-mono text-[11px] text-ink-4">{e.mentions}×</span>
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
                    <X className="size-3" />
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

function ClaimRow({ claim }: { claim: GraphClaim }) {
  return (
    <li className="flex items-start gap-3 rounded-card border border-line bg-card px-3 py-2.5">
      <span className="mt-0.5 shrink-0">
        {claim.grounded ? (
          <Check className="size-4 text-for" aria-label="grounded" />
        ) : (
          <Zap className="size-4 text-ink-4" aria-label="ungrounded" />
        )}
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-[13px] leading-snug text-ink-2">{claim.text}</p>
        <p className="mt-1 font-mono text-[11px] text-ink-4">
          {claim.sources} {claim.sources === 1 ? "source" : "sources"}
        </p>
      </div>
    </li>
  );
}

function ClaimList({ claims }: { claims: GraphClaim[] }) {
  return (
    <section>
      <SectionLabel>Related claims · {claims.length}</SectionLabel>
      {claims.length === 0 ? (
        <p className="text-[13px] text-ink-4">No claims connected to this page yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {claims.map((c) => (
            <ClaimRow key={c.id} claim={c} />
          ))}
        </ul>
      )}
    </section>
  );
}

function CiteRow({ cite }: { cite: GraphCite }) {
  return (
    <li className="flex items-start gap-3 rounded-card border border-line bg-card px-3 py-2.5">
      <Quote className="mt-0.5 size-4 shrink-0 text-ink-4" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-[13px] text-ink-2">{cite.title || cite.id}</p>
        <div className="mt-1 flex items-center gap-2 font-mono text-[11px] text-ink-4">
          {cite.provider ? <span>{cite.provider}</span> : null}
          {cite.sourceUrl ? (
            <span className="flex min-w-0 items-center gap-1">
              <Link2 className="size-3 shrink-0" />
              <span className="truncate">{cite.sourceUrl}</span>
            </span>
          ) : null}
        </div>
      </div>
    </li>
  );
}

function CiteList({ cites }: { cites: GraphCite[] }) {
  return (
    <section>
      <SectionLabel>Cites · {cites.length}</SectionLabel>
      {cites.length === 0 ? (
        <p className="text-[13px] text-ink-4">No cited sources yet.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {cites.map((c) => (
            <CiteRow key={c.id} cite={c} />
          ))}
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
        <FileText className="size-4 shrink-0 text-ink-4" />
        <span className="min-w-0 flex-1 truncate text-[13px] text-ink-2">{item.title || item.id}</span>
        <span className="shrink-0 rounded-chip border border-line px-1.5 py-0.5 font-mono text-[10px] text-ink-4">
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
        <p className="text-[13px] text-ink-4">No other artifacts connected yet.</p>
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
      {pane.thesis ? <ThesisCard thesis={pane.thesis} /> : null}
      <EntityList
        artifactId={artifactId}
        entities={pane.entities}
        onChanged={onChanged}
        onOpenEntity={onOpenEntity}
      />
      <LinkedArtifactList items={pane.linkedArtifacts} />
      <ClaimList claims={pane.claims} />
      <CiteList cites={pane.cites} />
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
        <span className="font-display text-[13px] text-ink">On the graph</span>
        <button
          type="button"
          aria-label="Close graph pane"
          onClick={onClose}
          className="ml-auto flex size-6 items-center justify-center rounded text-ink-3 transition-colors hover:bg-[rgb(var(--hover))] hover:text-ink"
        >
          <X className="size-4" />
        </button>
      </header>

      <div className="min-h-0 flex-1 overflow-auto px-3 py-4">
        <div className="flex flex-col gap-5">
          {/* Properties come from the artifact itself, so they render immediately —
              independent of the graph pane's load state. */}
          <PropertiesSection artifact={artifact} />
          {loading ? (
            <p className="text-[13px] text-ink-4">Loading…</p>
          ) : error ? (
            <p className="text-[13px] text-against">{error}</p>
          ) : pane ? (
            <PaneBody
              artifactId={artifactId}
              pane={pane}
              onChanged={reload}
              onOpenEntity={setPeekEntityId}
            />
          ) : null}
        </div>
      </div>
      <EntityPeek entityId={peekEntityId} onClose={() => setPeekEntityId(null)} />
    </aside>
  );
}
