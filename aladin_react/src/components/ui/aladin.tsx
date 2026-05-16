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
        "flex items-center gap-3 rounded-[10px] border border-[#e4e4e4] bg-white px-3.5 py-2.5 text-sm text-[#52525b]",
        className,
      )}
    >
      <Search className="h-4 w-4 text-[#111111]" />
      <span className="min-w-0 flex-1 truncate">{text}</span>
      <span className="eyebrow hidden sm:inline">Search</span>
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
    <div className={cn("flex h-full flex-col justify-between gap-8 bg-[#fcfcfb] px-8 py-8", className)}>
      <div className="space-y-4">
        <div className="eyebrow">Workspace</div>
        <h2 className="max-w-xl text-[2rem] font-semibold leading-[1.06] tracking-[-0.04em] text-[#111111]">
          {title}
        </h2>
        <p className="max-w-2xl text-sm leading-7 text-[#3f3f46]">{body}</p>
      </div>
      <div className="max-w-xl border-t border-[#e7e5e0] pt-5">
        <div className="eyebrow">Next</div>
        <p className="mt-3 text-sm leading-7 text-[#3f3f46]">
          Open an item from the workspace rail or create a new note to start building connected knowledge.
        </p>
      </div>
    </div>
  );
}
