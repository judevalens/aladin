import { Search } from "lucide-react";
import type { HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/lib/utils";

export function AladinShellPane({ className, children }: PropsWithChildren<{ className?: string }>) {
  return (
    <section className={cn("app-canvas flex h-full min-h-0 flex-col", className)}>
      {children}
    </section>
  );
}

export function AladinPanel({
  className,
  children,
  ...props
}: PropsWithChildren<HTMLAttributes<HTMLDivElement>>) {
  return (
    <div
      className={cn(
        "panel-soft",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function AladinToolbarField({ text, className }: { text: string; className?: string }) {
  return (
    <div
      className={cn(
        "flex h-8 items-center gap-2 rounded-md border border-line bg-field px-2.5 text-[12.5px] text-ink-3 transition-colors hover:border-ink-4 hover:text-ink-2",
        className,
      )}
    >
      <Search className="h-3.5 w-3.5" strokeWidth={1.75} />
      <span className="min-w-0 flex-1 truncate">{text}</span>
      <kbd className="hidden rounded border border-line bg-raise px-1 font-mono text-[10px] font-medium text-ink-3 sm:inline">⌘K</kbd>
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
    <div className={cn("flex h-full flex-col justify-center gap-3 bg-bg px-8 py-12", className)}>
      <div className="font-mono text-[10px] font-semibold uppercase tracking-[0.08em] text-ink-3">Workspace</div>
      <h2 className="max-w-lg font-display text-[1.375rem] font-semibold leading-[1.2] tracking-[-0.02em] text-ink">
        {title}
      </h2>
      <p className="max-w-md text-[13px] leading-[1.6] text-ink-2">{body}</p>
    </div>
  );
}
