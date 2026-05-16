import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/shared/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md border text-[13px] font-medium tracking-[-0.005em] transition-[background-color,color,border-color,box-shadow] duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563eb]/40 focus-visible:border-[#2563eb] active:translate-y-[0.5px] disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        default: "border-[#18181b] bg-[#18181b] text-[#fafaf9] shadow-[0_1px_0_rgba(0,0,0,0.04)] hover:bg-[#27272a]",
        secondary: "border-[#e7e5e4] bg-white text-[#0a0a0a] shadow-[0_1px_0_rgba(0,0,0,0.02)] hover:border-[#d6d3d1] hover:bg-[#f7f6f4]",
        ghost: "border-transparent bg-transparent text-[#57534e] hover:bg-[#ececea] hover:text-[#0a0a0a]",
        destructive: "border-[#dc2626] bg-white text-[#dc2626] hover:bg-[#fef2f2]",
      },
      size: {
        default: "h-9 px-3.5",
        sm: "h-7 px-2.5 text-[12px]",
        lg: "h-10 px-5",
        icon: "h-8 w-8 rounded-md",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button
      className={cn(buttonVariants({ variant, size }), className)}
      ref={ref}
      {...props}
    />
  ),
);
Button.displayName = "Button";
