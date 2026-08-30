import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "lucide-react";

export function BoardButton({ label, icon: Icon, active, ...props }: {
  label: string; icon: LucideIcon; active?: boolean;
} & ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type="button" className="rs-icon-button" aria-label={label} aria-pressed={active} {...props}>
    <Icon size={19} strokeWidth={1.65} />
  </button>;
}
