import { useEffect, useState } from "react";
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

/** Accepts a bare host ("example.com") or a full URL; returns a normalized URL or null. */
function normalizeUrl(raw: string): string | null {
  const trimmed = raw.trim();
  if (!trimmed) return null;
  const withScheme = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
  try {
    const parsed = new URL(withScheme);
    if (!parsed.hostname.includes(".")) return null;
    return parsed.toString();
  } catch {
    return null;
  }
}

export function LinkCaptureDialogUI({
  open,
  pending,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (url: string, title?: string) => Promise<void>;
}) {
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setUrl("");
      setTitle("");
      setError(null);
    }
  }, [open]);

  async function submit() {
    const normalized = normalizeUrl(url);
    if (!normalized) {
      setError("Enter a valid URL.");
      return;
    }
    await onSubmit(normalized, title.trim() || undefined);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,560px)]">
        <DialogHeader>
          <DialogTitle>New link</DialogTitle>
          <DialogDescription>Paste a URL to save it to your workspace.</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4 pb-4">
          <div className="space-y-2">
            <label className="eyebrow">URL</label>
            <Input
              autoFocus
              placeholder="https://…"
              value={url}
              onChange={(event) => {
                setUrl(event.target.value);
                setError(null);
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !pending) {
                  event.preventDefault();
                  void submit();
                }
              }}
            />
          </div>
          <div className="space-y-2">
            <label className="eyebrow">
              Title <span className="text-ink-4">(optional)</span>
            </label>
            <Input
              placeholder="Defaults to the link"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
            />
          </div>
          {error ? <p className="text-sm text-against">{error}</p> : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={pending || !url.trim()}>
            {pending ? "Saving…" : "Save link"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
