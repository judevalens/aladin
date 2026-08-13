import { Check, ChevronDown, Folder, Pause, Play } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useVoiceDraft } from "@/modules/workspace/hooks/use-workspace-state";
import { cn } from "@/lib/utils";

function formatDuration(ms: number): string {
  const total = Math.floor(ms / 1000);
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function VoiceCaptureDialogUI() {
  const {
    draft,
    permissionError,
    pending,
    elapsedMs,
    level,
    paused,
    folders,
    onClose,
    onPatchDraft,
    onStartRecording,
    onPauseRecording,
    onResumeRecording,
    onStopRecording,
    onSave,
  } = useVoiceDraft();
  const isRecording = draft?.phase === "recording";
  const selectedFolderId = draft?.folderId ?? null;
  const selectedFolder = folders.find((f) => f.id === selectedFolderId) ?? folders[0];

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
              <Eyebrow as="label">Title</Eyebrow>
              <Input
                value={draft.title}
                onChange={(event) => onPatchDraft({ title: event.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Eyebrow as="label">Notes</Eyebrow>
              <Textarea
                value={draft.description}
                onChange={(event) => onPatchDraft({ description: event.target.value })}
              />
            </div>

            <div className="space-y-2">
              <Eyebrow as="label">Save to</Eyebrow>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    type="button"
                    className="flex w-full items-center justify-between rounded-control border border-line bg-field px-3 py-2 text-body text-ink transition-colors hover:bg-raise"
                  >
                    <span className="flex items-center gap-2 truncate">
                      <Icon as={Folder} className="text-ink-3" />
                      {selectedFolder?.title ?? "Workspace"}
                    </span>
                    <Icon as={ChevronDown} className="text-ink-3" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start" className="max-h-64 w-[--radix-dropdown-menu-trigger-width] overflow-y-auto">
                  {folders.map((folder) => (
                    <DropdownMenuItem
                      key={folder.id ?? "root"}
                      onClick={() => onPatchDraft({ folderId: folder.id })}
                      style={{ paddingLeft: `${12 + folder.depth * 14}px` }}
                    >
                      <span className="flex-1 truncate">{folder.title}</span>
                      {folder.id === selectedFolderId ? (
                        <Icon as={Check} mark className="text-amber" />
                      ) : null}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>

            <div className="rounded-control border border-line bg-field p-4">
              {isRecording ? (
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-body text-ink-2">
                    <span className="flex items-center gap-2">
                      <span
                        className={cn(
                          "h-2.5 w-2.5 rounded-full",
                          paused ? "bg-ink-4" : "animate-pulse bg-against",
                        )}
                      />
                      {paused ? "Paused" : "Recording"}
                    </span>
                    <span className="font-mono tabular-nums text-ink">{formatDuration(elapsedMs)}</span>
                  </div>
                  {/* Live input level — reassures the user the mic is actually picking up sound. */}
                  <div className="h-1.5 overflow-hidden rounded-full bg-raise">
                    <div
                      className="h-full rounded-full bg-amber transition-[width] duration-75"
                      style={{ width: `${paused ? 0 : Math.min(100, Math.round(level * 140))}%` }}
                    />
                  </div>
                </div>
              ) : draft.audioUrl ? (
                <audio className="w-full" controls src={draft.audioUrl} />
              ) : (
                <p className="text-body leading-7 text-ink-3">
                  No audio captured yet. Start recording to create a preview.
                </p>
              )}
            </div>

            {permissionError || draft.errorMessage ? (
              <div className="space-y-3 rounded-control border border-against/40 bg-against/10 px-4 py-3 text-body leading-6 text-against">
                <p>{permissionError ?? draft.errorMessage}</p>
                {permissionError ? (
                  <Button variant="secondary" size="sm" onClick={onStartRecording}>
                    Try again
                  </Button>
                ) : null}
              </div>
            ) : null}
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          {isRecording ? (
            <>
              {paused ? (
                <Button variant="secondary" onClick={onResumeRecording}>
                  <Icon as={Play} />
                  Resume
                </Button>
              ) : (
                <Button variant="secondary" onClick={onPauseRecording}>
                  <Icon as={Pause} />
                  Pause
                </Button>
              )}
              <Button variant="destructive" onClick={onStopRecording}>
                Stop
              </Button>
            </>
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
