import { Search } from "lucide-react";
import type { HTMLAttributes, PropsWithChildren } from "react";
import { cn } from "@/shared/lib/utils";

export function AladinShellPane({ className, children }: PropsWithChildren<{ className?: string }>) {
  return (
    <section className={cn("flex h-full min-h-0 flex-col bg-white", className)}>
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
        "border border-gray-300 bg-white",
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
        "flex items-center gap-3 border border-gray-300 bg-white px-3 py-2 text-sm text-gray-700",
        className,
      )}
    >
      <Search className="h-4 w-4 text-black" />
      <span className="min-w-0 flex-1 truncate">{text}</span>
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
    <div className={cn("flex h-full flex-col gap-3 bg-white px-8 py-8", className)}>
      <h2 className="text-2xl font-semibold text-black">{title}</h2>
      <p className="max-w-2xl text-sm leading-6 text-gray-700">{body}</p>
    </div>
  );
}
