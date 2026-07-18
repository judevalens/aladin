import type { Platform } from "../entity-context-types";

// Platform chip per the Entity Context handoff §Plat — a small square with a
// mono label. Ink + tint are both derived from one per-platform hue via the
// handoff's oklch(0.82 0.08 <hue>) formula (from signals-shared.jsx), so no
// theme hexes are involved and the chip reads correctly in both themes.
const PLAT: Record<Platform, { label: string; hue: number }> = {
  x: { label: "X", hue: 210 },
  hn: { label: "HN", hue: 28 },
  reddit: { label: "RE", hue: 14 },
  rss: { label: "RS", hue: 285 },
  filing: { label: "SEC", hue: 150 },
  paper: { label: "AR", hue: 265 },
  // Not in the handoff's set — Aladin actually ingests Bluesky, and labelling it "X"
  // would be a lie. Its own hue, adjacent to X's (both microblogs).
  bluesky: { label: "BS", hue: 230 },
};

export function PlatChip({ p, size = 22 }: { p: Platform; size?: number }) {
  const meta = PLAT[p];
  // Unknown platform → render nothing (PRD §5, missing fields).
  if (!meta) return null;
  return (
    <span
      className="grid shrink-0 place-items-center rounded-sm font-mono font-bold tracking-[0.2px]"
      style={{
        width: size,
        height: size,
        fontSize: size >= 22 ? 9.5 : 8.5,
        color: `oklch(0.82 0.08 ${meta.hue})`,
        background: `oklch(0.82 0.08 ${meta.hue} / 0.14)`,
      }}
    >
      {meta.label}
    </span>
  );
}
