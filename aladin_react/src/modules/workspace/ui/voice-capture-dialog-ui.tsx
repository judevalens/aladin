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
import { Textarea } from "@/components/ui/textarea";
import { useVoiceDraft } from "@/modules/workspace/hooks/use-workspace-state";

export function VoiceCaptureDialogUI() {
  const {
    draft,
    permissionError,
    pending,
    onClose,
    onPatchDraft,
    onStartRecording,
    onStopRecording,
    onSave,
  } = useVoiceDraft();
  return (
    <Dialog open={Boolean(draft)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="w-[min(92vw,640px)]">
        <DialogHeader>
          <DialogTitle>New voice note</DialogTitle>
          <DialogDescription>Record audio, review it, then save it to the workspace.</DialogDescription>
        </DialogHeader>
        {draft ? (
          <DialogBody className="space-y-5 pb-4">
            <div className="space-y-2">
              <label className="eyebrow">Title</label>
              <Input
                value={draft.title}
                onChange={(event) => onPatchDraft({ title: event.target.value })}
              />
            </div>
            <div className="space-y-2">
              <label className="eyebrow">Notes</label>
              <Textarea
                value={draft.description}
                onChange={(event) => onPatchDraft({ description: event.target.value })}
              />
            </div>

            <div className="rounded-md border border-line bg-field p-4">
              {draft.phase === "recording" ? (
                <div className="flex items-center gap-3 text-sm text-ink-2">
                  <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-against" />
                  Recording in progress…
                </div>
              ) : draft.audioUrl ? (
                <audio className="w-full" controls src={draft.audioUrl} />
              ) : (
                <p className="text-sm leading-7 text-ink-3">
                  No audio captured yet. Start recording to create a preview.
                </p>
              )}
            </div>

            {permissionError || draft.errorMessage ? (
            <div className="rounded-md border border-against/40 bg-against/10 px-4 py-3 text-sm leading-6 text-against">
              {permissionError ?? draft.errorMessage}
            </div>
            ) : null}
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          {draft?.phase === "recording" ? (
            <Button variant="destructive" onClick={onStopRecording}>
              Stop recording
            </Button>
          ) : (
            <Button variant="secondary" onClick={onStartRecording}>
              {draft?.audioUrl ? "Record again" : "Start recording"}
            </Button>
          )}
          <Button onClick={onSave} disabled={pending || !draft?.audioBlob}>
            {pending ? "Saving…" : "Save voice note"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
