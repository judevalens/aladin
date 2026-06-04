import { useCallback, useEffect, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { BlockAttribution } from "@/repos/pages/page-attribution-repo";

// Block-level agent presence: the page's blockId -> { by, at } attribution map,
// fetched from the backend. `refetch` is called by the editor (debounced) when
// the doc changes, so an agent's edit shows up shortly after it lands.
export function usePageAttribution(pageId: string): {
  attribution: BlockAttribution;
  refetch: () => void;
} {
  const { repos } = useAppComposition();
  const [attribution, setAttribution] = useState<BlockAttribution>({});

  const refetch = useCallback(() => {
    void repos.pages.getAttribution(pageId).then(setAttribution);
  }, [repos, pageId]);

  useEffect(() => {
    refetch();
  }, [refetch]);

  return { attribution, refetch };
}
