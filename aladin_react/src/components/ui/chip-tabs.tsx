import { cn } from "@/lib/utils";

/**
 * Chip-style tab strip — the filter/sort control used across Aladin's index surfaces.
 * Lifted from insights-ui's local `Tabs` (which inlines the same classes) so the
 * Entities index doesn't become a third copy. Those two can migrate onto it later.
 */
export function ChipTabs<T extends string>({
  tabs,
  active,
  onSelect,
  className,
}: {
  tabs: { key: T; label: string }[];
  active: T;
  onSelect: (key: T) => void;
  className?: string;
}) {
  return (
    <div className={cn("flex items-center gap-1", className)}>
      {tabs.map((tab) => (
        <button
          key={tab.key}
          type="button"
          onClick={() => onSelect(tab.key)}
          className={cn(
            "rounded-chip px-2.5 py-1 text-[12px] transition-colors",
            active === tab.key
              ? "bg-[rgb(var(--sel))] text-ink"
              : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
          )}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
}
