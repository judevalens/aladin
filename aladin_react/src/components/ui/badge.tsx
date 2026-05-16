import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/shared/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-full border px-2 py-0.5 text-[10.5px] font-medium tracking-[0.005em]",
  {
    variants: {
      variant: {
        default: "border-[#e7e5e4] bg-[#fafaf9] text-[#57534e]",
        muted: "border-[#e7e5e4] bg-[#f2f0ee] text-[#57534e]",
        inverted: "border-[#18181b] bg-[#18181b] text-[#fafaf9]",
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
