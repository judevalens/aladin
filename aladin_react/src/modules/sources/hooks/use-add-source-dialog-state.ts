import { useState } from "react";

export interface AddSourceDialogState {
  streamQuery: string;
  streamTitle: string;
  streamLimit: string;
  streamErrorMessage: string | null;
  createSourcePending: boolean;
  onStreamQueryChange: (value: string) => void;
  onStreamTitleChange: (value: string) => void;
  onStreamLimitChange: (value: string) => void;
  onCreateSource: () => Promise<void>;
  onOpenChange: (open: boolean) => void;
}

export function useAddSourceDialogState({
  open,
  onOpenChange,
  createSource,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  createSource: (input: {
    query: string;
    title: string;
    limit: string;
  }) => Promise<void>;
}): AddSourceDialogState {
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
    streamQuery,
    streamTitle,
    streamLimit,
    streamErrorMessage,
    createSourcePending,
    onOpenChange: (nextOpen: boolean) => {
      onOpenChange(nextOpen);
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
        onOpenChange(false);
      } catch (error) {
        setStreamErrorMessage(error instanceof Error ? error.message : "Failed to create stream.");
      } finally {
        setCreateSourcePending(false);
      }
    },
  };
}
