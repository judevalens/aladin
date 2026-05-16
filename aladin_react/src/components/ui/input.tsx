import * as React from "react";
import { cn } from "@/shared/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, ...props }, ref) => (
    <input
      ref={ref}
      className={cn(
        "flex h-11 w-full rounded-[10px] border border-[#e4e4e4] bg-white px-3.5 py-2 text-sm text-[#111111] transition-[border-color,background-color,box-shadow] placeholder:text-[#8b8b8b] focus-visible:border-[#202020] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#202020]/10 disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";
