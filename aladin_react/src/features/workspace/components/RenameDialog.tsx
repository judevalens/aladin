import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { api } from "@/shared/api/client";
import { browserTreeQueryKey } from "@/features/workspace/queries";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";

export function RenameDialog() {
  const queryClient = useQueryClient();
  const { state, dispatch } = useWorkspaceShell();
  const rename = state.activeRename;

  const mutation = useMutation({
    mutationFn: async () => {
      if (!rename) return;
      if (rename.kind === "folder") {
        await api.updateFolder(rename.rowId, { title: rename.draftTitle.trim() });
      } else {
        await api.updateUserArtifact(rename.rowId, { title: rename.draftTitle.trim() });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
      if (rename?.kind === "artifact") {
        await queryClient.invalidateQueries({ queryKey: ["artifact", rename.rowId] });
      }
      dispatch({ type: "cancel-rename" });
    },
  });

  return (
    <Dialog
      open={Boolean(rename)}
      onOpenChange={(open) => {
        if (!open) dispatch({ type: "cancel-rename" });
      }}
    >
      <DialogContent className="w-[min(92vw,520px)]">
        <DialogHeader>
          <DialogTitle>Rename {rename?.kind === "folder" ? "folder" : "artifact"}</DialogTitle>
          <DialogDescription>Update the title in place.</DialogDescription>
        </DialogHeader>
        <DialogBody className="pb-2">
          <Input
            autoFocus
            value={rename?.draftTitle ?? ""}
            onChange={(event) =>
              dispatch({ type: "set-rename-title", title: event.target.value })
            }
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => dispatch({ type: "cancel-rename" })}>
            Cancel
          </Button>
          <Button
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending || !rename?.draftTitle.trim()}
          >
            {mutation.isPending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
