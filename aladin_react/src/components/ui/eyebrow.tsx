import type { ElementType, HTMLAttributes } from "react";

import { cn } from "@/lib/utils";

/**
 * The small uppercase mono label above a section — "WHAT'S IN HERE", "HYPOTHESIS", "SOURCES".
 *
 * There is a `.eyebrow` class in index.css and roughly 30 call sites, and the audit found
 * **7 different treatments** anyway (10px/.12em · 11px/.14em · 10.5px/.5px · 9.5px/.4px/bold
 * · 10px/.7px · 10px/.4px · the class itself). A class cannot stop that, because a class is
 * easy to not use. A component is what people reach for.
 *
 * `tone="loud"` is §5 rule 4 (amber is the only accent — spend it) applied to labels: at most
 * one per surface, and only when that section is the one needing attention.
 *
 * `as` exists because these labels are not all block-level section headings: plenty sit inline
 * inside a flex row next to their value ("DAY RANGE  184.10 – 187.62"). Forcing a `div` there
 * would change the layout, which is how a primitive gets abandoned. Default stays `div`.
 */
export function Eyebrow({
  className,
  tone = "quiet",
  as: Tag = "div",
  ...props
}: HTMLAttributes<HTMLElement> & {
  tone?: "quiet" | "loud";
  as?: Extract<ElementType, "div" | "span" | "p" | "h2" | "h3" | "h4">;
}) {
  return (
    <Tag
      className={cn(
        "font-mono text-meta font-semibold uppercase tracking-[0.08em]",
        tone === "quiet" ? "text-ink-3" : "text-amber",
        className,
      )}
      {...props}
    />
  );
}
