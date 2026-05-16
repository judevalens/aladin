import { Search } from "lucide-react";
import type { HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/shared/lib/utils";

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
        "flex h-8 items-center gap-2 rounded-md border border-[#e7e5e4] bg-[#fafaf9] px-2.5 text-[12.5px] text-[#a8a29e] transition-colors hover:border-[#d6d3d1] hover:bg-white",
        className,
      )}
    >
      <Search className="h-3.5 w-3.5" strokeWidth={1.75} />
      <span className="min-w-0 flex-1 truncate">{text}</span>
      <kbd className="hidden rounded border border-[#e7e5e4] bg-white px-1 text-[10px] font-medium text-[#a8a29e] sm:inline">⌘K</kbd>
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
    <div className={cn("flex h-full flex-col justify-center gap-3 bg-[#fafaf9] px-8 py-12", className)}>
      <div className="eyebrow">Workspace</div>
      <h2 className="max-w-lg text-[1.375rem] font-semibold leading-[1.2] tracking-[-0.02em] text-[#0a0a0a]">
        {title}
      </h2>
      <p className="max-w-md text-[13px] leading-[1.6] text-[#57534e]">{body}</p>
    </div>
  );
}
