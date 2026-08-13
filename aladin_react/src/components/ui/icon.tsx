import type { LucideIcon, LucideProps } from "lucide-react";

/**
 * The only way to render a lucide glyph in this app.
 *
 * Before this existed the audit found 2 APIs (`className="size-4"` vs a `size={14}` prop),
 * 5 sizes and **13 stroke weights** across 62 component files — because both size and stroke
 * were free parameters at every call site, and a free parameter drifts. Three sizes and two
 * strokes, chosen by name, remove the choice.
 *
 * See `design/UI_ARCHITECTURE.md` §5 rule 9: never set `strokeWidth` or a raw pixel size at
 * a call site.
 */

const SIZES = {
  /** 14 — beside text: tab icons, chip glyphs, chevrons in rows */
  inline: 14,
  /** 16 — icon buttons, tree type icons, menu items (absorbs the old 15 and 17) */
  default: 16,
  /** 18 — the 38px rail cells only; nothing else earns 18 */
  rail: 18,
} as const;

export type IconSize = keyof typeof SIZES;

export function Icon({
  as: Glyph,
  size = "default",
  mark = false,
  ...rest
}: {
  as: LucideIcon;
  size?: IconSize;
  /**
   * Marks — chevrons, checks, `×`. They need stroke 2 to hold their shape at 12–14px.
   * Everything else is 1.75, which is the app's line weight.
   */
  mark?: boolean;
} & Omit<LucideProps, "size">) {
  return <Glyph size={SIZES[size]} strokeWidth={mark ? 2 : 1.75} {...rest} />;
}
