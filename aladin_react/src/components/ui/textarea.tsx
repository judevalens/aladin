import * as React from "react";
import { cn } from "@/shared/lib/utils";

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      "flex min-h-[112px] w-full rounded-md border border-[#e7e5e4] bg-white px-3 py-2.5 text-[13px] leading-6 text-[#0a0a0a] transition-[border-color,background-color,box-shadow] placeholder:text-[#a8a29e] focus-visible:border-[#2563eb] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563eb]/20 disabled:cursor-not-allowed disabled:opacity-50",
      className,
    )}
    {...props}
  />
));
Textarea.displayName = "Textarea";
