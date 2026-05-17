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
import type { AddSourceDialogProps } from "@/modules/sources/ui/sources-view-types";

export function AddSourceDialog(props: AddSourceDialogProps) {
  const {
    open,
    streamQuery,
    streamTitle,
    streamLimit,
    streamErrorMessage,
    createSourcePending,
    onOpenChange,
    onStreamQueryChange,
    onStreamTitleChange,
    onStreamLimitChange,
    onCreateSource,
  } = props;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,640px)]">
        <DialogHeader>
          <DialogTitle>Add source</DialogTitle>
          <DialogDescription>
            Create a new search stream for this workspace.
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-5 pb-4">
          <div className="space-y-2">
            <label className="eyebrow">Provider</label>
            <div className="rounded-md border border-[#e7e5e4] bg-white px-4 py-3 text-sm text-[#0a0a0a]">
              Bluesky search
            </div>
          </div>
          <div className="space-y-2">
            <label className="eyebrow">Query</label>
            <Input
              value={streamQuery}
              onChange={(event) => onStreamQueryChange(event.target.value)}
              placeholder="e.g. blocknote OR yjs"
            />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="eyebrow">Display title</label>
              <Input
                value={streamTitle}
                onChange={(event) => onStreamTitleChange(event.target.value)}
                placeholder="Optional custom title"
              />
            </div>
            <div className="space-y-2">
              <label className="eyebrow">Limit</label>
              <Input
                value={streamLimit}
                onChange={(event) => onStreamLimitChange(event.target.value)}
              />
            </div>
          </div>
          {streamErrorMessage ? (
            <div className="rounded-md border border-[#e7e5e4] bg-white px-4 py-3 text-sm leading-6 text-[#0a0a0a]">
              {streamErrorMessage}
            </div>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={createSourcePending || streamQuery.trim().length === 0}
            onClick={onCreateSource}
          >
            {createSourcePending ? "Creating…" : "Create stream"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
