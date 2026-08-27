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

/** How many rows the bulk manifest shows by name before collapsing into "+ N more". */
const MANIFEST_LIMIT = 6;

/**
 * Confirmation for a delete — one target or a whole selection.
 *
 * Deleting is the one destructive thing the tree can do, and the tree's rows are small and
 * close together — a right-click landing one row off is easy. So it asks, and it says what
 * goes, by name. A bulk delete lists its manifest (folders with their descendant counts, so
 * the true blast radius is visible) and calls out folder recursion explicitly — that's the
 * one surprise a batch can hide.
 *
 * It does NOT promise permanence, because the server doesn't do that: the delete is a
 * tombstone on the tree node and the artifact body is retained. "Removed from your workspace"
 * is the honest phrasing, and it leaves room for an undo later without the copy becoming a lie.
 */
export function DeleteConfirmDialog({
  targets,
  onCancel,
  onConfirm,
}: {
  targets: DeleteTarget[] | null;
  onCancel: () => void;
  onConfirm: (targets: DeleteTarget[]) => Promise<void>;
}) {
  useOrphanedPointerLockGuard();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const close = () => {
    setError(null);
    onCancel();
  };

  const confirm = async () => {
    if (!targets?.length) return;
    setPending(true);
    setError(null);
    try {
      await onConfirm(targets);
      close();
    } catch (cause) {
      // Stay open on failure. Closing here would read as success. A partial bulk failure
      // lands here too — the message names what survived.
      setError(cause instanceof Error ? cause.message : "Couldn't delete that.");
    } finally {
      setPending(false);
    }
  };

  const many = (targets?.length ?? 0) > 1;
  const single = targets?.length === 1 ? targets[0] : null;
  const count = single?.childCount ?? 0;
  const folders = targets?.filter((t) => t.kind !== "artifact") ?? [];

  return (
    <Dialog open={Boolean(targets?.length)} onOpenChange={(open) => !open && !pending && close()}>
      <DialogContent className="w-[min(92vw,460px)]">
        <DialogHeader>
          <DialogTitle>
            {many ? `Delete ${targets!.length} items?` : `Delete ${single ? NOUN[single.kind] : "item"}?`}
          </DialogTitle>
          <DialogDescription>
            {many ? (
              "They'll be removed from your workspace on every device."
            ) : (
              <>
                <span className="text-ink-2">{single?.title}</span>
                {count > 0
                  ? ` and everything inside it (${count} item${count === 1 ? "" : "s"}) will be removed from your workspace.`
                  : " will be removed from your workspace."}
              </>
            )}
          </DialogDescription>
        </DialogHeader>
        {many ? (
          <DialogBody className="flex flex-col gap-2.5 pb-2">
            <div className="rounded-control border border-line-2 bg-card p-1">
              {targets!.slice(0, MANIFEST_LIMIT).map((target) => (
                <div
                  key={`${target.kind}:${target.id}`}
                  className="flex h-[26px] items-center gap-2 rounded-tap px-2 text-small text-ink-2"
                >
                  <span className="min-w-0 flex-1 truncate">{target.title}</span>
                  <span className="shrink-0 font-mono text-meta text-ink-3">
                    {target.kind === "artifact"
                      ? null
                      : `${NOUN[target.kind]}${
                          target.childCount
                            ? ` · ${target.childCount} item${target.childCount === 1 ? "" : "s"} inside`
                            : ""
                        }`}
                  </span>
                </div>
              ))}
              {targets!.length > MANIFEST_LIMIT ? (
                <div className="px-2 pt-1 pb-1.5 font-mono text-meta text-ink-3">
                  + {targets!.length - MANIFEST_LIMIT} more
                </div>
              ) : null}
            </div>
            {folders.length > 0 ? (
              <p className="rounded-control border border-against/40 bg-against/10 px-3 py-2 text-small text-against">
                {folders.length === 1
                  ? `"${folders[0].title}" is a folder — everything inside it is deleted too.`
                  : `${folders.length} of these are folders — everything inside them is deleted too.`}
              </p>
            ) : null}
          </DialogBody>
        ) : null}
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
            {pending ? "Deleting…" : many ? `Delete ${targets!.length} items` : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
