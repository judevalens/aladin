import { useNavigate, useParams } from "react-router-dom";

import { EntitySurface } from "@/modules/entities/ui/entity-surface";

/**
 * The Entity Context surface as a full-page route (/entity/:entityId) — the permalink /
 * deep-link view. The index opens entities in a modal peek; this page is reached by a
 * direct URL or a tag-bar chip. Edge clicks navigate, so the router's history is the
 * back-trail.
 */
export function EntityContextRoute() {
  const { entityId = "" } = useParams();
  const navigate = useNavigate();
  return (
    <div className="h-full w-full overflow-hidden">
      <EntitySurface entityId={entityId} onOpenEntity={(id) => navigate(`/entity/${id}`)} />
    </div>
  );
}
