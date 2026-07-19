import { useId } from "react";

// Build an SVG path `d` from a series, scaled to the given box.
function pathFor(series: number[], w: number, h: number, pad = 1): { line: string; area: string } {
  if (series.length === 0) return { line: "", area: "" };
  const min = Math.min(...series);
  const max = Math.max(...series);
  const span = max - min || 1;
  const stepX = (w - pad * 2) / Math.max(series.length - 1, 1);
  const pts = series.map((v, i) => {
    const x = pad + i * stepX;
    const y = pad + (h - pad * 2) * (1 - (v - min) / span);
    return [x, y] as const;
  });
  const line = pts.map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(2)} ${y.toFixed(2)}`).join(" ");
  const area = `${line} L${pts[pts.length - 1][0].toFixed(2)} ${h} L${pts[0][0].toFixed(2)} ${h} Z`;
  return { line, area };
}

/** A tiny row sparkline. `up` picks the semantic color. */
export function Sparkline({ series, up, width = 64, height = 22 }: { series: number[]; up: boolean; width?: number; height?: number }) {
  const { line } = pathFor(series, width, height);
  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className={up ? "text-for" : "text-against"} aria-hidden>
      <path d={line} fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

/** The detail-panel area chart: filled gradient under a trend line. */
export function AreaChart({ series, up, height = 190 }: { series: number[]; up: boolean; height?: number }) {
  const gradId = useId();
  const w = 640;
  const { line, area } = pathFor(series, w, height, 4);
  return (
    <svg
      viewBox={`0 0 ${w} ${height}`}
      preserveAspectRatio="none"
      className={`w-full ${up ? "text-for" : "text-against"}`}
      style={{ height }}
      aria-hidden
    >
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="currentColor" stopOpacity={0.28} />
          <stop offset="100%" stopColor="currentColor" stopOpacity={0} />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${gradId})`} stroke="none" />
      <path d={line} fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}
