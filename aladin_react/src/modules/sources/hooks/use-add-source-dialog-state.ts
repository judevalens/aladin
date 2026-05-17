import { useState } from "react";

export function useAddSourceDialogState({
  createSource,
}: {
  createSource: (input: {
    query: string;
    title: string;
    limit: string;
  }) => Promise<void>;
}) {
  const [open, setOpen] = useState(false);
  const [streamQuery, setStreamQuery] = useState("");
  const [streamTitle, setStreamTitle] = useState("");
  const [streamLimit, setStreamLimit] = useState("25");
  const [streamErrorMessage, setStreamErrorMessage] = useState<string | null>(null);
  const [createSourcePending, setCreateSourcePending] = useState(false);

  const resetDraft = () => {
    setStreamQuery("");
    setStreamTitle("");
    setStreamLimit("25");
    setStreamErrorMessage(null);
  };

  return {
    open,
    streamQuery,
    streamTitle,
    streamLimit,
    streamErrorMessage,
    createSourcePending,
    openDialog: () => setOpen(true),
    onOpenChange: (nextOpen: boolean) => {
      setOpen(nextOpen);
      if (!nextOpen) {
        resetDraft();
      }
    },
    onStreamQueryChange: setStreamQuery,
    onStreamTitleChange: setStreamTitle,
    onStreamLimitChange: setStreamLimit,
    onCreateSource: async () => {
      try {
        setCreateSourcePending(true);
        setStreamErrorMessage(null);
        await createSource({
          query: streamQuery,
          title: streamTitle,
          limit: streamLimit,
        });
        resetDraft();
        setOpen(false);
      } catch (error) {
        setStreamErrorMessage(error instanceof Error ? error.message : "Failed to create stream.");
      } finally {
        setCreateSourcePending(false);
      }
    },
  };
}
