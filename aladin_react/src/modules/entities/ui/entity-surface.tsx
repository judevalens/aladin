import { useState } from "react";

import { useEntityContext } from "@/modules/entities/hooks/use-entity-context";
import { DrawConnectionUI } from "@/modules/entities/ui/draw-connection-ui";
import { EntityContextView } from "@/modules/entities/ui/entity-context-ui";
import { MergeReviewUI } from "@/modules/entities/ui/merge-review-ui";

// Calm skeleton for the surface's lists — no decorative spinners (PRD §5).
function EntitySurfaceSkeleton() {
  return (
    <div className="mx-auto max-w-[820px] px-[30px] pt-[30px] pb-[60px]">
      <div className="mb-3 h-3 w-64 animate-pulse rounded bg-line" />
      <div className="mb-[10px] h-8 w-96 animate-pulse rounded bg-line" />
      <div className="mb-[26px] h-4 w-[520px] animate-pulse rounded bg-line-2" />
      <div className="flex flex-col gap-2">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-[62px] animate-pulse rounded-[10px] bg-card" />
        ))}
      </div>
    </div>
  );
}

/**
 * The composed Entity Context surface — hook + presentational view + the draw-connection
 * and merge-review slots. Shared by the full-page route and the modal peek; the only
 * difference is `onOpenEntity` (the route navigates, the peek swaps in place), so
 * "pulling the thread" works in whichever container is hosting it.
 */
export function EntitySurface({
  entityId,
  onOpenEntity,
}: {
  entityId: string;
  onOpenEntity: (id: string) => void;
}) {
  const {
    entity, edges, context, merges, dataPoints, externalIds, company,
    loading, error, drawEdge, acceptMerge, rejectMerge,
  } = useEntityContext(entityId);
  const [drawing, setDrawing] = useState(false);

  if (loading) return <EntitySurfaceSkeleton />;

  if (error || !entity) {
    return (
      <div className="mx-auto max-w-[820px] px-[30px] pt-[30px]">
        <p className="text-[13px] text-ink-2">{error ?? "That entity could not be found."}</p>
      </div>
    );
  }

  return (
    <EntityContextView
      entity={entity}
      edges={edges}
      context={context}
      dataPoints={dataPoints}
      externalIds={externalIds}
      company={company}
      onOpenEntity={onOpenEntity}
      onDrawConnection={() => setDrawing(true)}
      mergeSlot={<MergeReviewUI merges={merges} onAccept={acceptMerge} onReject={rejectMerge} />}
      drawSlot={
        drawing ? (
          <DrawConnectionUI
            selfId={entity.id}
            onCancel={() => setDrawing(false)}
            onConfirm={async (edge) => {
              await drawEdge(edge);
              setDrawing(false);
            }}
          />
        ) : null
      }
    />
  );
}
