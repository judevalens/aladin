import { useEffect } from "react";
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
import { useRenameDialog } from "@/modules/workspace/hooks/use-workspace-state";

export function RenameDialogUI() {
  const { rename, pending, onDraftTitleChange, onCancel, onSave } = useRenameDialog();

  // Radix bug workaround: when a Dialog is opened from a (modal) ContextMenu, the
  // dialog can capture the menu's `pointer-events: none` as its "original" body
  // value and restore that `none` on close — leaving the whole app unclickable
  // even though nothing is open. Watch for a residual lock and strip it, but ONLY
  // when no modal layer is actually mounted, so an open menu/dialog keeps its
  // legitimate lock. This reacts to the real DOM mutation (no timers).
  useEffect(() => {
    const root = document.documentElement;
    const body = document.body;
    const stripOrphanedLock = () => {
      if (body.style.pointerEvents !== "none" && root.style.pointerEvents !== "none") return;
      if (document.querySelector("[role='dialog'],[role='alertdialog'],[role='menu']")) return;
      body.style.pointerEvents = "";
      root.style.pointerEvents = "";
    };
    const observer = new MutationObserver(stripOrphanedLock);
    observer.observe(body, { attributes: true, attributeFilter: ["style"], childList: true });
    observer.observe(root, { attributes: true, attributeFilter: ["style"] });
    return () => observer.disconnect();
  }, []);

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
