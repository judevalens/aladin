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
import { useAddSourceDialogState } from "@/modules/sources/hooks/use-add-source-dialog-state";
import type { AddSourceDialogProps } from "@/modules/sources/ui/sources-ui-types";

export function AddSourceDialog(props: AddSourceDialogProps) {
  const { open, onOpenChange, createSource } = props;
  const state = useAddSourceDialogState({
    open,
    onOpenChange,
    createSource,
  });

  return (
    <Dialog open={open} onOpenChange={state.onOpenChange}>
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
            <div className="rounded-md border border-line bg-field px-4 py-3 text-sm text-ink">
              Bluesky search
            </div>
          </div>
          <div className="space-y-2">
            <label className="eyebrow">Query</label>
            <Input
              value={state.streamQuery}
              onChange={(event) => state.onStreamQueryChange(event.target.value)}
              placeholder="e.g. blocknote OR yjs"
            />
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <label className="eyebrow">Display title</label>
              <Input
                value={state.streamTitle}
                onChange={(event) => state.onStreamTitleChange(event.target.value)}
                placeholder="Optional custom title"
              />
            </div>
            <div className="space-y-2">
              <label className="eyebrow">Limit</label>
              <Input
                value={state.streamLimit}
                onChange={(event) => state.onStreamLimitChange(event.target.value)}
              />
            </div>
          </div>
          {state.streamErrorMessage ? (
            <div className="rounded-md border border-against/40 bg-against/10 px-4 py-3 text-sm leading-6 text-against">
              {state.streamErrorMessage}
            </div>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => state.onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={state.createSourcePending || state.streamQuery.trim().length === 0}
            onClick={state.onCreateSource}
          >
            {state.createSourcePending ? "Creating…" : "Create stream"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
