import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/shared/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2.5 py-1 text-[11px] font-medium tracking-[0.01em]",
  {
    variants: {
      variant: {
        default: "border-[#e4e4e4] bg-white text-[#52525b]",
        muted: "border-[#e4e4e4] bg-[#f3f3f3] text-[#616161]",
        inverted: "border-[#ededed] bg-[#ededed] text-[#111111]",
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
