import { useMemo, useState } from "react";
import type { Source } from "@/shared/api/models";

export function useSourceDetailsDialogState({
  sources,
  removeSource,
}: {
  sources: Source[];
  removeSource: (sourceId: string) => Promise<void>;
}) {
  const [selectedSourceId, setSelectedSourceId] = useState<string | null>(null);
  const [removeSourcePending, setRemoveSourcePending] = useState(false);

  const selectedSource = useMemo(
    () => sources.find((source) => source.id === selectedSourceId) ?? null,
    [selectedSourceId, sources],
  );

  return {
    selectedSource,
    removeSourcePending,
    onSelectSource: setSelectedSourceId,
    onRemoveSelectedSource: async () => {
      if (!selectedSource) return;
      try {
        setRemoveSourcePending(true);
        await removeSource(selectedSource.id);
        setSelectedSourceId(null);
      } finally {
        setRemoveSourcePending(false);
      }
    },
  };
}
