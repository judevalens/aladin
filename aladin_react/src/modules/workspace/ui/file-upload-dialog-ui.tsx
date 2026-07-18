import { useEffect, useRef, useState } from "react";
import { UploadCloud } from "lucide-react";
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
import { cn } from "@/lib/utils";

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function FileUploadDialogUI({
  open,
  pending,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (file: File, title?: string) => Promise<void>;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!open) {
      setFile(null);
      setDragging(false);
    }
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,560px)]">
        <DialogHeader>
          <DialogTitle>Upload file</DialogTitle>
          <DialogDescription>Drop a file, or choose one from your machine.</DialogDescription>
        </DialogHeader>
        <DialogBody className="pb-4">
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => {
              event.preventDefault();
              setDragging(false);
              const dropped = event.dataTransfer.files?.[0];
              if (dropped) setFile(dropped);
            }}
            className={cn(
              "flex w-full flex-col items-center gap-2 rounded-md border border-dashed px-6 py-10 text-center transition-colors",
              dragging ? "border-amber bg-amber-soft" : "border-line bg-field hover:bg-raise",
            )}
          >
            <UploadCloud className="size-6 text-ink-3" strokeWidth={1.6} />
            {file ? (
              <span className="text-sm text-ink">
                {file.name} <span className="text-ink-4">({formatBytes(file.size)})</span>
              </span>
            ) : (
              <span className="text-sm text-ink-3">Drop a file here, or click to choose</span>
            )}
          </button>
          <input
            ref={inputRef}
            type="file"
            className="hidden"
            onChange={(event) => {
              const chosen = event.target.files?.[0];
              if (chosen) setFile(chosen);
            }}
          />
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={() => file && onSubmit(file)} disabled={pending || !file}>
            {pending ? "Uploading…" : "Upload"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
