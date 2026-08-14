import { useState } from "react";

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
import { useOrphanedPointerLockGuard } from "@/modules/workspace/hooks/use-orphaned-pointer-lock";

export interface DeleteTarget {
  kind: "folder" | "research" | "artifact";
  id: string;
  title: string;
  /** Folders only: how many things go with it. Omitted when the folder is empty. */
  childCount?: number;
}

const NOUN: Record<DeleteTarget["kind"], string> = {
  folder: "folder",
  research: "research folder",
  artifact: "item",
};

/**
 * Confirmation for a delete.
 *
 * Deleting is the one destructive thing the tree can do, and the tree's rows are small and
 * close together — a right-click landing one row off is easy. So it asks, and it says what
 * goes, by name.
 *
 * It does NOT promise permanence, because the server doesn't do that: the delete is a
 * tombstone on the tree node and the artifact body is retained. "Removed from your workspace"
 * is the honest phrasing, and it leaves room for an undo later without the copy becoming a lie.
 */
export function DeleteConfirmDialog({
  target,
  onCancel,
  onConfirm,
}: {
  target: DeleteTarget | null;
  onCancel: () => void;
  onConfirm: (target: DeleteTarget) => Promise<void>;
}) {
  useOrphanedPointerLockGuard();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setError(null);
    onCancel();
  };

  const confirm = async () => {
    if (!target) return;
    setPending(true);
    setError(null);
    try {
      await onConfirm(target);
      close();
    } catch (cause) {
      // Stay open on failure. Closing here would read as success.
      setError(cause instanceof Error ? cause.message : "Couldn't delete that.");
    } finally {
      setPending(false);
    }
  };

  const noun = target ? NOUN[target.kind] : "item";
  const count = target?.childCount ?? 0;

  return (
    <Dialog open={Boolean(target)} onOpenChange={(open) => !open && !pending && close()}>
      <DialogContent className="w-[min(92vw,460px)]">
        <DialogHeader>
          <DialogTitle>Delete {noun}?</DialogTitle>
          <DialogDescription>
            <span className="text-ink-2">{target?.title}</span>
            {count > 0
              ? ` and everything inside it (${count} item${count === 1 ? "" : "s"}) will be removed from your workspace.`
              : " will be removed from your workspace."}
          </DialogDescription>
        </DialogHeader>
        {error ? (
          <DialogBody className="pb-2">
            <p className="rounded-control border border-against/40 bg-against/10 px-3 py-2 text-small text-against">
              {error}
            </p>
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={close} disabled={pending}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={confirm} disabled={pending}>
            {pending ? "Deleting…" : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
