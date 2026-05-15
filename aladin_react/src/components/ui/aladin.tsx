import { Search } from "lucide-react";
import type { PropsWithChildren } from "react";
import { cn } from "@/shared/lib/utils";

export function AladinShellPane({ className, children }: PropsWithChildren<{ className?: string }>) {
  return (
    <section className={cn("flex h-full min-h-0 flex-col bg-aladin-canvas", className)}>
      {children}
    </section>
  );
}

export function AladinPanel({ className, children }: PropsWithChildren<{ className?: string }>) {
  return (
    <div
      className={cn(
        "rounded-sharp border border-aladin-divider bg-aladin-panel",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function AladinToolbarField({ text, className }: { text: string; className?: string }) {
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-control border border-aladin-border bg-aladin-command-surface px-3.5 py-2.5 text-sm text-aladin-ink-secondary",
        className,
      )}
    >
      <Search className="h-4 w-4 text-aladin-ink" />
      <span>{text}</span>
    </div>
  );
}

export function PlaceholderPane({
  title,
  body,
  className,
}: {
  title: string;
  body: string;
  className?: string;
}) {
  return (
    <div className={cn("flex h-full flex-col gap-2 bg-aladin-panel p-6", className)}>
      <h2 className="text-3xl font-semibold tracking-[-0.04em] text-aladin-ink">{title}</h2>
      <p className="max-w-2xl text-sm leading-6 text-aladin-ink-secondary">{body}</p>
    </div>
  );
}
