import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
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
import { api, toArtifact } from "@/shared/api/client";
import { browserTreeQueryKey } from "@/features/workspace/queries";
import { useWorkspaceShell } from "@/features/workspace/workspace-state";

export function VoiceCaptureDialog() {
  const queryClient = useQueryClient();
  const { state, dispatch } = useWorkspaceShell();
  const draft = state.activeVoiceDraft;
  const recorderRef = useRef<MediaRecorder | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const [permissionError, setPermissionError] = useState<string | null>(null);

  const uploadMutation = useMutation({
    mutationFn: async () => {
      if (!draft?.audioBlob) {
        throw new Error("Record audio before saving.");
      }
      const filename = `${draft.title.trim().replace(/\s+/g, "-").toLowerCase() || "voice-note"}.webm`;
      return api.uploadUserArtifact({
        type: "voice",
        file: draft.audioBlob,
        filename,
        title: draft.title.trim(),
        summary: draft.description.trim() || undefined,
        folderId: draft.folderId,
      });
    },
    onSuccess: async (record) => {
      await queryClient.invalidateQueries({ queryKey: browserTreeQueryKey });
      dispatch({ type: "close-voice-draft" });
      dispatch({ type: "open-artifact", artifactId: toArtifact(record).id });
    },
    onError: (error) => {
      dispatch({
        type: "patch-voice-draft",
        patch: {
          phase: "failed",
          errorMessage: error instanceof Error ? error.message : "Failed to save voice note.",
        },
      });
    },
  });

  useEffect(() => {
    return () => {
      recorderRef.current?.stop();
      streamRef.current?.getTracks().forEach((track) => track.stop());
      if (draft?.audioUrl) {
        URL.revokeObjectURL(draft.audioUrl);
      }
    };
  }, [draft?.audioUrl]);

  async function startRecording() {
    if (!navigator.mediaDevices?.getUserMedia) {
      setPermissionError("Audio capture is not available in this browser.");
      return;
    }

    try {
      setPermissionError(null);
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      streamRef.current = stream;
      chunksRef.current = [];
      const recorder = new MediaRecorder(stream);
      recorderRef.current = recorder;
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) chunksRef.current.push(event.data);
      };
      recorder.onstop = () => {
        const blob = new Blob(chunksRef.current, {
          type: recorder.mimeType || "audio/webm",
        });
        const audioUrl = URL.createObjectURL(blob);
        dispatch({
          type: "patch-voice-draft",
          patch: {
            phase: "review",
            audioBlob: blob,
            audioUrl,
            errorMessage: null,
          },
        });
        stream.getTracks().forEach((track) => track.stop());
        streamRef.current = null;
        recorderRef.current = null;
      };
      recorder.start();
      dispatch({ type: "patch-voice-draft", patch: { phase: "recording", errorMessage: null } });
    } catch (error) {
      setPermissionError(error instanceof Error ? error.message : "Failed to start recording.");
    }
  }

  function stopRecording() {
    recorderRef.current?.stop();
  }

  return (
    <Dialog
      open={Boolean(draft)}
      onOpenChange={(open) => {
        if (!open) {
          dispatch({ type: "close-voice-draft" });
        }
      }}
    >
      <DialogContent className="w-[min(92vw,640px)]">
        <DialogHeader>
          <DialogTitle>New voice note</DialogTitle>
          <DialogDescription>Record audio, review it, then save it to the workspace.</DialogDescription>
        </DialogHeader>
        {draft ? (
          <DialogBody className="space-y-4 pb-4">
            <div className="space-y-2">
              <label className="text-xs text-gray-500">
                Title
              </label>
              <Input
                value={draft.title}
                onChange={(event) =>
                  dispatch({
                    type: "patch-voice-draft",
                    patch: { title: event.target.value },
                  })
                }
              />
            </div>
            <div className="space-y-2">
              <label className="text-xs text-gray-500">
                Notes
              </label>
              <Textarea
                value={draft.description}
                onChange={(event) =>
                  dispatch({
                    type: "patch-voice-draft",
                    patch: { description: event.target.value },
                  })
                }
              />
            </div>

            <div className="border border-gray-300 bg-white p-4">
              {draft.phase === "recording" ? (
                <div className="flex items-center gap-3 text-sm text-gray-700">
                  <span className="h-2.5 w-2.5 animate-pulse rounded-full bg-red-500" />
                  Recording in progress…
                </div>
              ) : draft.audioUrl ? (
                <audio className="w-full" controls src={draft.audioUrl} />
              ) : (
                <p className="text-sm text-gray-500">
                  No audio captured yet. Start recording to create a preview.
                </p>
              )}
            </div>

            {permissionError || draft.errorMessage ? (
              <div className="border border-gray-300 bg-white px-3 py-2 text-sm text-black">
                {permissionError ?? draft.errorMessage}
              </div>
            ) : null}
          </DialogBody>
        ) : null}
        <DialogFooter>
          <Button variant="secondary" onClick={() => dispatch({ type: "close-voice-draft" })}>
            Cancel
          </Button>
          {draft?.phase === "recording" ? (
            <Button variant="destructive" onClick={stopRecording}>
              Stop recording
            </Button>
          ) : (
            <Button variant="secondary" onClick={startRecording}>
              {draft?.audioUrl ? "Record again" : "Start recording"}
            </Button>
          )}
          <Button
            onClick={() => uploadMutation.mutate()}
            disabled={uploadMutation.isPending || !draft?.audioBlob}
          >
            {uploadMutation.isPending ? "Saving…" : "Save voice note"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
