import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/shared/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center border px-2 py-0.5 text-[11px] font-medium",
  {
    variants: {
      variant: {
        default: "border-gray-300 bg-gray-100 text-gray-700",
        muted: "border-gray-300 bg-white text-gray-500",
        inverted: "border-black bg-black text-white",
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
