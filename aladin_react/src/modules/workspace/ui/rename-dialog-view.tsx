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
import type { RenameDraft } from "@/modules/workspace/domain";
import type { RenameDialogState } from "@/modules/workspace/hooks/use-workspace";

export function RenameDialogView({ state }: { state: RenameDialogState }) {
  const { rename, pending, onDraftTitleChange, onCancel, onSave } = state;
  return (
    <Dialog open={Boolean(rename)} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent className="w-[min(92vw,520px)]">
        <DialogHeader>
          <DialogTitle>Rename {rename?.kind === "folder" ? "folder" : "artifact"}</DialogTitle>
          <DialogDescription>Update the title in place without changing its location.</DialogDescription>
        </DialogHeader>
        <DialogBody className="pb-2">
          <Input
            autoFocus
            value={rename?.draftTitle ?? ""}
            onChange={(event) => onDraftTitleChange(event.target.value)}
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={onCancel}>
            Cancel
          </Button>
          <Button onClick={onSave} disabled={pending || !rename?.draftTitle.trim()}>
            {pending ? "Saving…" : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
