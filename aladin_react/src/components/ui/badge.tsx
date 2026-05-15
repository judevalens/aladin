import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/shared/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-[0.2em]",
  {
    variants: {
      variant: {
        default: "border-aladin-border bg-aladin-panel text-aladin-code-text",
        muted: "border-aladin-divider bg-aladin-panel-muted text-aladin-ink-muted",
        inverted: "border-aladin-ink bg-aladin-ink text-aladin-onInkSurface",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

export function Badge({
  className,
  variant,
  ...props
}: HTMLAttributes<HTMLDivElement> & VariantProps<typeof badgeVariants>) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}
