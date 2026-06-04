import { useCallback, useEffect, useState } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import type { PageEditEntry } from "@/repos/pages/page-attribution-repo";

// Page edit history (humans + agents), newest first. Reads from the API only —
// never touches the editor DOM.
export function usePageHistory(pageId: string): {
  entries: PageEditEntry[];
  loading: boolean;
  refetch: () => void;
} {
  const { repos } = useAppComposition();
  const [entries, setEntries] = useState<PageEditEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const refetch = useCallback(() => {
    setLoading(true);
    void repos.pages.getHistory(pageId).then((e) => {
      setEntries(e);
      setLoading(false);
    });
  }, [repos, pageId]);

  useEffect(() => {
    refetch();
  }, [refetch]);

  return { entries, loading, refetch };
}
