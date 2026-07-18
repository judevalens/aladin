import { Tag, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import { useAppComposition } from "@/app/composition/app-composition";
import type { AttachedEntity } from "@/modules/graph/graph-pane-types";

// The page-level entity tag bar: shows the entities a page is linked to (manual `tag`s,
// removable, plus read-only projected `@mention`s). Adding/creating entity tags from here
// was removed — tagging is handled by the properties layer now.
export function EntityTagBar({ pageId }: { pageId: string }) {
  const { repos } = useAppComposition();
  const navigate = useNavigate();
  const [entities, setEntities] = useState<AttachedEntity[]>([]);

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

  const tags = entities.filter((e) => e.origin === "tag");
  // A mention already tagged shouldn't render twice; show only mention-origin entities not
  // also tagged.
  const tagIds = new Set(tags.map((t) => t.id));
  const mentions = entities.filter((e) => e.origin === "mention" && !tagIds.has(e.id));

  // No add affordance anymore — hide the bar entirely when the page has no links.
  if (tags.length === 0 && mentions.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-1.5 px-8 pb-1 pt-2">
      <Tag className="h-3.5 w-3.5 text-ink-4" />
      {tags.map((e) => (
        <span
          key={`tag-${e.id}`}
          className="group flex items-center gap-1 rounded-chip bg-amber-soft px-1.5 py-0.5 text-xs font-medium text-amber"
          title={e.kind}
        >
          <button
            onClick={() => navigate(`/entity/${e.id}`)}
            title="Open entity context"
            className="cursor-pointer hover:underline"
          >
            {e.name}
          </button>
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
        <button
          key={`men-${e.id}`}
          onClick={() => navigate(`/entity/${e.id}`)}
          className="cursor-pointer rounded-chip bg-raise px-1.5 py-0.5 text-xs text-ink-3 hover:text-ink"
          title={`@mention · ${e.kind} — open entity context`}
        >
          @{e.name}
        </button>
      ))}
    </div>
  );
}
