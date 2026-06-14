// @aladin/kit — the shard authoring kit (KIT-1 core).
//
// Thin stable core: identity (Region), opaque-origin-safe hash routing, layout
// chrome, and the Aladin semantic hues. Generic UI (buttons, cards, dialogs)
// comes from shadcn/Radix re-exports in a later increment. Every component is
// Tailwind/token-styled and reactivity-agnostic; semantic components compose
// Region rather than reimplementing anchoring.
//
// Built once (esbuild, react externalized → shared instance) and served
// content-addressed at /vendor/<sha>; agents `import { … } from "@aladin/kit"`.

import { useEffect, useState } from "react";
import type { ReactNode } from "react";

export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

// --- L0 identity ------------------------------------------------------------

export type RegionKind =
  | "collection"
  | "metric"
  | "chart"
  | "narrative"
  | "control"
  | (string & {});

export interface RegionProps {
  anchor: string;
  kind?: RegionKind;
  className?: string;
  children?: ReactNode;
}

// Region marks an addressable surface — it stamps data-anchor (+ data-kind) so
// the region is identifiable for feedback, deep links, and graph ingestion.
// Identity is a primitive: semantic components compose Region.
export function Region({ anchor, kind, className, children }: RegionProps) {
  return (
    <div data-anchor={anchor} data-kind={kind} className={className}>
      {children}
    </div>
  );
}

// --- L1 routing (opaque-origin-safe hash routing) ---------------------------

export function useRoute(): string {
  const read = () =>
    typeof window === "undefined" ? "/" : window.location.hash.slice(1) || "/";
  const [route, setRoute] = useState(read);
  useEffect(() => {
    const on = () => setRoute(read());
    window.addEventListener("hashchange", on);
    return () => window.removeEventListener("hashchange", on);
  }, []);
  return route;
}

export function HashRouter({ children }: { children?: ReactNode }) {
  return <>{children}</>;
}

export function Route({ path, children }: { path: string; children?: ReactNode }) {
  return useRoute() === path ? <>{children}</> : null;
}

export function Link({
  to,
  className,
  children,
}: {
  to: string;
  className?: string;
  children?: ReactNode;
}) {
  return (
    <a href={"#" + to} className={className}>
      {children}
    </a>
  );
}

// --- L1 layout / chrome -----------------------------------------------------

interface Styled {
  className?: string;
  children?: ReactNode;
}

export function Page({ className, children }: Styled) {
  return <div className={cn("min-h-screen bg-bg font-sans text-ink", className)}>{children}</div>;
}

export function Section({ className, children }: Styled) {
  return <section className={cn("mx-auto max-w-3xl px-8 py-10", className)}>{children}</section>;
}

export function Panel({ className, children }: Styled) {
  return <div className={cn("rounded-card border border-line bg-panel p-5", className)}>{children}</div>;
}

export function Toolbar({ className, children }: Styled) {
  return (
    <div className={cn("flex items-center gap-2 border-b border-line bg-chrome px-4 py-2", className)}>
      {children}
    </div>
  );
}

// --- L2 semantic hues (research vocabulary) ---------------------------------

export function For({ className, children }: Styled) {
  return <span className={cn("text-for", className)}>{children}</span>;
}
export function Against({ className, children }: Styled) {
  return <span className={cn("text-against", className)}>{children}</span>;
}
export function Catalyst({ className, children }: Styled) {
  return <span className={cn("text-catalyst", className)}>{children}</span>;
}
export function Echo({ className, children }: Styled) {
  return <span className={cn("text-echo", className)}>{children}</span>;
}

// --- L3 viz theming (token-resolved colors for charts / SVG) -----------------
//
// var(--color-*) resolves at runtime (the shard inlines the tokens into :root),
// but it is INERT inside an SVG presentation attribute (stroke="…" / fill="…"),
// which is exactly how recharts and hand-drawn SVG set colors. tok() flattens a
// token to a CONCRETE color so it's safe to pass as an attribute; the chart*
// helpers return spread-ready, on-theme props for the recharts primitives. Import
// recharts yourself; the kit only supplies the theme glue (no recharts coupling).

// tok resolves a design token (e.g. "--color-amber") to a concrete CSS color by
// reading a throwaway element's computed `color`, flattening the whole var()
// chain to an rgb(...) string — valid anywhere, including SVG attributes.
export function tok(name: string): string {
  if (typeof document === "undefined") return "#888";
  const el = document.createElement("span");
  el.style.color = "var(" + name + ")";
  el.style.display = "none";
  document.body.appendChild(el);
  const c = getComputedStyle(el).color;
  el.remove();
  return c || "#888";
}

// chartSeries is the ordered on-theme palette for chart series, resolved to
// concrete colors. Call at render time (it reads computed styles); index/cycle as
// needed: <Bar fill={chartSeries()[0]} />.
export function chartSeries(): string[] {
  return [
    "--color-amber",
    "--color-echo",
    "--color-for",
    "--color-against",
    "--color-catalyst",
    "--color-ink-3",
  ].map(tok);
}

// chartAxis / chartGrid / chartTooltip return spread-ready, theme-resolved props
// for the matching recharts primitives — call them in render:
//   <XAxis {...chartAxis()} dataKey="x" />
//   <CartesianGrid {...chartGrid()} />
//   <Tooltip {...chartTooltip()} />
export function chartAxis() {
  const ink = tok("--color-ink-3");
  return {
    stroke: ink,
    tick: { fill: ink, fontSize: 11 },
    tickLine: { stroke: tok("--color-line-2") },
  };
}
export function chartGrid() {
  return { stroke: tok("--color-line-2"), strokeDasharray: "3 3" };
}
export function chartTooltip() {
  return {
    contentStyle: {
      background: tok("--color-card"),
      border: "1px solid " + tok("--color-line"),
      borderRadius: 8,
      color: tok("--color-ink"),
      fontSize: 12,
    },
    cursor: { fill: tok("--color-raise") },
    labelStyle: { color: tok("--color-ink-2") },
  };
}
