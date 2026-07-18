import { useEffect, useState } from "react";

import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { EntitySurface } from "@/modules/entities/ui/entity-surface";

/**
 * The entity "peek" — a quick-look modal over the index (and anywhere else), so you can
 * inspect an entity and act on its merge questions without leaving where you were. Esc /
 * click-away / the ✕ close it (all free from Radix).
 *
 * Thread-pulling stays inside the modal: clicking an edge swaps the peeked entity rather
 * than navigating, and closing returns you to the list. The /entity/:id route is the
 * permalink version of the same surface for deep links.
 */
export function EntityPeek({
  entityId,
  onClose,
}: {
  entityId: string | null;
  onClose: () => void;
}) {
  // Track the entity being viewed so edge clicks can walk the graph within the modal.
  const [current, setCurrent] = useState<string | null>(entityId);
  useEffect(() => setCurrent(entityId), [entityId]);

  return (
    <Dialog open={current !== null} onOpenChange={(next) => !next && onClose()}>
      {current !== null ? (
        <DialogContent className="h-[min(88vh,880px)] w-[min(94vw,1040px)] p-0">
          {/* Radix requires a title for a11y; the surface renders the visible identity. */}
          <DialogTitle className="sr-only">Entity details</DialogTitle>
          <div className="min-h-0 flex-1 overflow-hidden">
            <EntitySurface entityId={current} onOpenEntity={setCurrent} />
          </div>
        </DialogContent>
      ) : null}
    </Dialog>
  );
}
