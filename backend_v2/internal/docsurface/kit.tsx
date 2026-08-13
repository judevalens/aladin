// @aladin/kit — the shard authoring kit (KIT-1 core).
//
// Layers: L0 identity (Region), L1 routing + layout chrome, L2 semantic hues,
// L3 chart/SVG theming helpers, L4 generic UI (Button/Card/Field/Tabs/Dialog…).
// Generic UI is kit-native (React + Tailwind tokens only — no Radix/shadcn
// vendoring), so it adds no bundle weight and inherits the theme. Every component
// is token-styled and reactivity-agnostic; semantic components compose Region
// rather than reimplementing anchoring.
//
// Built once (esbuild, react externalized → shared instance) and served
// content-addressed at /vendor/<sha>; agents `import { … } from "@aladin/kit"`.

import { useEffect, useState, useSyncExternalStore } from "react";
import type {
  ReactNode,
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  TextAreaHTMLAttributes,
} from "react";

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

// --- L4 generic UI (token-styled, dependency-free) ---------------------------
//
// Kit-native primitives for the things agents otherwise hand-roll (buttons,
// cards, fields, tabs, dialogs). Built on React + Tailwind tokens only — no
// Radix/shadcn vendoring, so they add nothing to a shard's bundle weight and
// inherit the Aladin theme automatically.

type ButtonVariant = "primary" | "outline" | "ghost" | "danger";
type ButtonSize = "sm" | "md";
const btnBase =
  "inline-flex items-center justify-center gap-2 rounded-chip font-mono transition-colors " +
  "focus:outline-none focus-visible:ring-2 focus-visible:ring-amber-line disabled:opacity-50 disabled:pointer-events-none";
const btnVariant: Record<ButtonVariant, string> = {
  primary: "bg-amber text-bg hover:bg-amber-soft",
  outline: "border border-line text-ink hover:bg-raise",
  ghost: "text-ink-2 hover:text-ink hover:bg-raise",
  danger: "bg-against text-bg hover:opacity-90",
};
const btnSize: Record<ButtonSize, string> = {
  sm: "h-7 px-2 text-[11px]",
  md: "h-9 px-3 text-sm",
};
export function Button({
  variant = "primary",
  size = "md",
  className,
  ...rest
}: { variant?: ButtonVariant; size?: ButtonSize } & ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button className={cn(btnBase, btnVariant[variant], btnSize[size], className)} {...rest} />;
}

export function Card({ className, children }: Styled) {
  return <div className={cn("rounded-card border border-line bg-card p-4", className)}>{children}</div>;
}

export function Divider({ className }: { className?: string }) {
  return <hr className={cn("border-0 border-t border-line", className)} />;
}

type BadgeTone = "neutral" | "amber" | "for" | "against";
export function Badge({
  tone = "neutral",
  className,
  children,
}: { tone?: BadgeTone; className?: string; children?: ReactNode }) {
  const tones: Record<BadgeTone, string> = {
    neutral: "bg-raise text-ink-2 border-line",
    amber: "text-amber border-amber-line",
    for: "text-for border-line",
    against: "text-against border-line",
  };
  return (
    <span className={cn("inline-flex items-center rounded-chip border px-1.5 py-0.5 font-mono text-[11px]", tones[tone], className)}>
      {children}
    </span>
  );
}

const fieldCls =
  "w-full rounded-chip border border-line bg-field px-2.5 py-1.5 text-sm text-ink " +
  "placeholder:text-ink-4 focus:outline-none focus:border-amber-line";
export function Input({ className, ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn(fieldCls, className)} {...rest} />;
}
export function Textarea({ className, ...rest }: TextAreaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cn(fieldCls, "min-h-20", className)} {...rest} />;
}

export function Field({
  label,
  hint,
  htmlFor,
  className,
  children,
}: { label?: ReactNode; hint?: ReactNode; htmlFor?: string; className?: string; children?: ReactNode }) {
  return (
    <div className={cn("flex flex-col gap-1", className)}>
      {label ? <label htmlFor={htmlFor} className="font-mono text-xs text-ink-3">{label}</label> : null}
      {children}
      {hint ? <p className="font-mono text-[11px] text-ink-4">{hint}</p> : null}
    </div>
  );
}

type CalloutTone = "info" | "warn" | "for" | "against";
export function Callout({
  tone = "info",
  title,
  className,
  children,
}: { tone?: CalloutTone; title?: ReactNode; className?: string; children?: ReactNode }) {
  const border: Record<CalloutTone, string> = {
    info: "border-line",
    warn: "border-amber-line",
    for: "border-line",
    against: "border-line",
  };
  const accent: Record<CalloutTone, string> = {
    info: "text-ink",
    warn: "text-amber",
    for: "text-for",
    against: "text-against",
  };
  return (
    <div className={cn("rounded-card border bg-panel p-3", border[tone], className)}>
      {title ? <div className={cn("mb-1 font-display text-sm", accent[tone])}>{title}</div> : null}
      <div className="text-sm text-ink-2">{children}</div>
    </div>
  );
}

export function Stat({
  label,
  value,
  sub,
  className,
}: { label?: ReactNode; value?: ReactNode; sub?: ReactNode; className?: string }) {
  return (
    <div className={cn("rounded-card border border-line bg-card p-3", className)}>
      {label ? <div className="font-mono text-[11px] uppercase tracking-wide text-ink-3">{label}</div> : null}
      <div className="font-display text-2xl text-ink">{value}</div>
      {sub ? <div className="font-mono text-[11px] text-ink-4">{sub}</div> : null}
    </div>
  );
}

// Tabs is a self-contained tablist: pass [{id,label,content}]; it tracks the
// active tab internally. For controlled use, drive your own state + conditional.
export function Tabs({
  tabs,
  initialId,
  className,
}: { tabs: Array<{ id: string; label: ReactNode; content: ReactNode }>; initialId?: string; className?: string }) {
  const [active, setActive] = useState(initialId ?? (tabs[0] ? tabs[0].id : ""));
  const current = tabs.find((t) => t.id === active) ?? tabs[0];
  return (
    <div className={className}>
      <div className="flex gap-1 border-b border-line" role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            aria-selected={t.id === active}
            onClick={() => setActive(t.id)}
            className={cn(
              "-mb-px border-b-2 px-3 py-1.5 font-mono text-sm transition-colors",
              t.id === active ? "border-amber text-amber" : "border-transparent text-ink-3 hover:text-ink",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div role="tabpanel" className="pt-4">{current ? current.content : null}</div>
    </div>
  );
}

// Dialog is a controlled modal: render it with open + onClose. Backdrop click and
// Escape both close. Opaque-origin-safe (no portals needed — it's fixed-position).
export function Dialog({
  open,
  onClose,
  title,
  className,
  children,
}: { open: boolean; onClose: () => void; title?: ReactNode; className?: string; children?: ReactNode }) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4" role="dialog" aria-modal="true">
      <div className="absolute inset-0 bg-bg/70" onClick={onClose} />
      <div className={cn("relative z-10 w-full max-w-lg rounded-modal border border-line bg-raise p-5 shadow-modal", className)}>
        {title ? <div className="mb-3 font-display text-lg text-ink">{title}</div> : null}
        {children}
      </div>
    </div>
  );
}

// --- L4b data display --------------------------------------------------------
//
// The read-out components every data shard otherwise hand-rolls. All
// dependency-free and token-styled; charts stay with recharts (install_lib) —
// these cover the tabular/at-a-glance cases where a chart library is overkill.

export interface Column<T> {
  key: string;
  label: ReactNode;
  /** Cell renderer; defaults to String(row[key]). */
  render?: (row: T) => ReactNode;
  align?: "left" | "right";
  width?: string;
}

/**
 * DataTable — a token-styled table. Each row carries data-aladin-key={rowKey},
 * so rows are addressable from the outside (deep links, verification) without
 * the shard doing anything extra.
 */
export function DataTable<T>({
  columns,
  rows,
  rowKey,
  onRowClick,
  empty,
  className,
}: {
  columns: Array<Column<T>>;
  rows: T[];
  rowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  empty?: ReactNode;
  className?: string;
}) {
  if (rows.length === 0) {
    return <>{empty ?? <EmptyState title="Nothing to show" />}</>;
  }
  return (
    <div className={cn("overflow-x-auto rounded-card border border-line", className)}>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="border-b border-line bg-raise">
            {columns.map((c) => (
              <th
                key={c.key}
                style={c.width ? { width: c.width } : undefined}
                className={cn(
                  "px-3 py-2 font-mono text-[11px] font-normal uppercase tracking-wide text-ink-3",
                  c.align === "right" ? "text-right" : "text-left",
                )}
              >
                {c.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr
              key={rowKey(row)}
              data-aladin-key={rowKey(row)}
              onClick={onRowClick ? () => onRowClick(row) : undefined}
              className={cn(
                "border-b border-line-2 last:border-0",
                onRowClick && "cursor-pointer hover:bg-raise",
              )}
            >
              {columns.map((c) => (
                <td
                  key={c.key}
                  className={cn("px-3 py-2 text-ink", c.align === "right" && "text-right tabular-nums")}
                >
                  {c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? "")}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** KeyValue — a definition list for entity/detail views. */
export function KeyValue({
  items,
  className,
}: {
  items: Array<{ label: ReactNode; value: ReactNode; hint?: string }>;
  className?: string;
}) {
  return (
    <dl className={cn("grid grid-cols-[minmax(6rem,auto)_1fr] gap-x-4 gap-y-2 text-sm", className)}>
      {items.map((item, i) => (
        <div key={i} className="contents">
          <dt className="font-mono text-[11px] uppercase tracking-wide text-ink-3" title={item.hint}>
            {item.label}
          </dt>
          <dd className="text-ink">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

/** Delta — a signed number in the semantic up/down hues. */
export function Delta({
  value,
  suffix = "",
  className,
}: {
  value: number;
  suffix?: string;
  className?: string;
}) {
  const tone = value > 0 ? "text-for" : value < 0 ? "text-against" : "text-ink-3";
  const sign = value > 0 ? "+" : "";
  return (
    <span className={cn("font-mono tabular-nums", tone, className)}>
      {sign}
      {value}
      {suffix}
    </span>
  );
}

/** MetricRow — a row of headline numbers, each with an optional delta. */
export function MetricRow({
  metrics,
  className,
}: {
  metrics: Array<{ label: string; value: ReactNode; delta?: number; hint?: string }>;
  className?: string;
}) {
  return (
    <div className={cn("grid gap-3", className)} style={{ gridTemplateColumns: `repeat(${Math.min(metrics.length, 4)}, minmax(0, 1fr))` }}>
      {metrics.map((m) => (
        <div key={m.label} className="rounded-card border border-line bg-card p-3" title={m.hint}>
          <div className="font-mono text-[11px] uppercase tracking-wide text-ink-3">{m.label}</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="font-display text-xl text-ink">{m.value}</span>
            {m.delta !== undefined && <Delta value={m.delta} />}
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * Sparkline — an inline trend line, hand-drawn SVG (no chart dependency).
 * Colors resolve through useTheme so a theme switch repaints it.
 */
export function Sparkline({
  points,
  width = 120,
  height = 28,
  tone,
  className,
}: {
  points: number[];
  width?: number;
  height?: number;
  tone?: "for" | "against" | "amber";
  className?: string;
}) {
  useTheme(); // re-resolve tok() colors when the theme changes
  if (points.length < 2) return <svg width={width} height={height} className={className} />;
  const min = Math.min(...points);
  const max = Math.max(...points);
  const span = max - min || 1;
  const dx = width / (points.length - 1);
  const d = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${(i * dx).toFixed(2)},${(height - ((p - min) / span) * height).toFixed(2)}`)
    .join(" ");
  const auto = points[points.length - 1] >= points[0] ? "for" : "against";
  const stroke = tok("text-" + (tone ?? auto));
  return (
    <svg width={width} height={height} className={className} aria-hidden="true">
      <path d={d} fill="none" stroke={stroke} strokeWidth="1.5" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

/** ProgressBar — a bounded 0..max meter. */
export function ProgressBar({
  value,
  max = 100,
  label,
  className,
}: {
  value: number;
  max?: number;
  label?: ReactNode;
  className?: string;
}) {
  const pct = Math.max(0, Math.min(100, (value / (max || 1)) * 100));
  return (
    <div className={className}>
      {label && (
        <div className="mb-1 flex justify-between font-mono text-[11px] text-ink-3">
          <span>{label}</span>
          <span className="tabular-nums">{Math.round(pct)}%</span>
        </div>
      )}
      <div
        className="h-1.5 w-full overflow-hidden rounded-chip bg-field"
        role="progressbar"
        aria-valuenow={value}
        aria-valuemin={0}
        aria-valuemax={max}
      >
        <div className="h-full bg-amber transition-[width]" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}

// --- L4c app chrome + forms --------------------------------------------------

/**
 * AppShell — the standard mini-app frame: a hash-routed sidebar plus content.
 * Pair with HashRouter/Route; nav entries link to the same hash routes the
 * manifest declares.
 */
export function AppShell({
  title,
  nav,
  footer,
  className,
  children,
}: {
  title?: ReactNode;
  nav?: Array<{ id: string; label: ReactNode; to: string }>;
  footer?: ReactNode;
  className?: string;
  children?: ReactNode;
}) {
  const route = useRoute();
  return (
    <div className={cn("flex min-h-screen bg-bg text-ink", className)}>
      <aside className="flex w-52 shrink-0 flex-col border-r border-line bg-rail p-3">
        {title && <div className="mb-3 px-2 font-display text-sm text-ink">{title}</div>}
        <nav className="flex flex-col gap-0.5">
          {(nav ?? []).map((item) => (
            <a
              key={item.id}
              href={item.to}
              className={cn(
                "rounded-chip px-2 py-1.5 text-sm transition-colors",
                route === item.to ? "bg-amber-soft text-amber" : "text-ink-2 hover:bg-raise hover:text-ink",
              )}
            >
              {item.label}
            </a>
          ))}
        </nav>
        {footer && <div className="mt-auto px-2 pt-3 text-[11px] text-ink-4">{footer}</div>}
      </aside>
      <main className="min-w-0 flex-1 overflow-auto p-5">{children}</main>
    </div>
  );
}

/** SearchInput — a labeled filter box with a clear affordance. */
export function SearchInput({
  value,
  onChange,
  placeholder = "Search…",
  className,
}: {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <div className={cn("relative", className)}>
      <Input
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
        className="pr-7"
      />
      {value && (
        <button
          type="button"
          aria-label="Clear search"
          onClick={() => onChange("")}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-ink-4 hover:text-ink"
        >
          ×
        </button>
      )}
    </div>
  );
}

/** Select — the native control, token-styled. */
export function Select({
  options,
  value,
  onChange,
  className,
}: {
  options: Array<{ value: string; label: string }>;
  value: string;
  onChange: (next: string) => void;
  className?: string;
}) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value)} className={cn(fieldCls, "pr-8", className)}>
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  );
}

export function Checkbox({
  checked,
  onChange,
  label,
  className,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label?: ReactNode;
  className?: string;
}) {
  return (
    <label className={cn("inline-flex cursor-pointer items-center gap-2 text-sm text-ink", className)}>
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-3.5 w-3.5 accent-[var(--amber)]"
      />
      {label}
    </label>
  );
}

export function RadioGroup({
  name,
  options,
  value,
  onChange,
  className,
}: {
  name: string;
  options: Array<{ value: string; label: ReactNode }>;
  value: string;
  onChange: (next: string) => void;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-1.5", className)} role="radiogroup">
      {options.map((o) => (
        <label key={o.value} className="inline-flex cursor-pointer items-center gap-2 text-sm text-ink">
          <input
            type="radio"
            name={name}
            value={o.value}
            checked={value === o.value}
            onChange={() => onChange(o.value)}
            className="h-3.5 w-3.5 accent-[var(--amber)]"
          />
          {o.label}
        </label>
      ))}
    </div>
  );
}

export function EmptyState({
  title,
  hint,
  action,
  className,
}: {
  title: ReactNode;
  hint?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("rounded-card border border-dashed border-line p-8 text-center", className)}>
      <div className="text-sm text-ink-2">{title}</div>
      {hint && <div className="mt-1 text-[12px] text-ink-4">{hint}</div>}
      {action && <div className="mt-3">{action}</div>}
    </div>
  );
}

export function LoadingState({ label = "Loading…", className }: { label?: string; className?: string }) {
  return (
    <div className={cn("flex items-center gap-2 p-6 text-sm text-ink-3", className)}>
      <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-amber" />
      {label}
    </div>
  );
}

// Toasts — a tiny module-level bus so any component can raise one without
// threading props. <Toasts/> renders them; mount it once near the app root.
type Toast = { id: number; message: ReactNode; tone: "neutral" | "for" | "against" };
let _toastSeq = 0;
const _toasts: Toast[] = [];
const _toastSubs = new Set<() => void>();

function emitToasts() {
  _toastSubs.forEach((fn) => fn());
}

export function useToast() {
  return {
    show(message: ReactNode, tone: Toast["tone"] = "neutral", ms = 3000) {
      const toast: Toast = { id: ++_toastSeq, message, tone };
      _toasts.push(toast);
      emitToasts();
      setTimeout(() => {
        const i = _toasts.findIndex((t) => t.id === toast.id);
        if (i >= 0) {
          _toasts.splice(i, 1);
          emitToasts();
        }
      }, ms);
    },
  };
}

export function Toasts({ className }: { className?: string }) {
  const toasts = useSyncExternalStore(
    (onChange) => {
      _toastSubs.add(onChange);
      return () => _toastSubs.delete(onChange);
    },
    () => _toasts.length, // a version key; the array itself is stable
    () => 0,
  );
  void toasts;
  if (_toasts.length === 0) return null;
  return (
    <div className={cn("fixed bottom-4 right-4 z-50 flex flex-col gap-2", className)}>
      {_toasts.map((t) => (
        <div
          key={t.id}
          className={cn(
            "animate-pop rounded-card border bg-raise px-3 py-2 text-sm shadow-toast",
            t.tone === "for" ? "border-line text-for" : t.tone === "against" ? "border-line text-against" : "border-line text-ink",
          )}
        >
          {t.message}
        </div>
      ))}
    </div>
  );
}

// --- L4d interactive / stateful ----------------------------------------------
//
// The interactive pieces a teaching or tracking shard needs. Each takes an
// optional stateKey: with it the component persists through useShardState (so
// progress survives reload and follows the user across clients); without it the
// state is in-memory. Data in, fixed behavior out — the determinism lever that
// makes a regenerated shard behave like the original.

/** usePersisted picks persistent or local state from the same call site. */
function usePersisted<T>(stateKey: string | undefined, initial: T): [T, (next: T | ((prev: T) => T)) => void] {
  const [local, setLocal] = useState<T>(initial);
  const [stored, setStored] = useShardState<T>(stateKey ?? "__unused__", initial);
  return stateKey ? [stored, setStored] : [local, setLocal];
}

export interface QuizQuestion {
  id: string;
  prompt: ReactNode;
  choices: Array<{ id: string; text: ReactNode }>;
  answerId: string;
  explanation?: ReactNode;
}

/** Quiz — answer, get graded, see why. Persists answers when stateKey is set. */
export function Quiz({
  questions,
  stateKey,
  onComplete,
  className,
}: {
  questions: QuizQuestion[];
  stateKey?: string;
  onComplete?: (score: number) => void;
  className?: string;
}) {
  const [answers, setAnswers] = usePersisted<Record<string, string>>(stateKey, {});
  const answered = questions.filter((q) => answers[q.id]).length;
  const score = questions.filter((q) => answers[q.id] === q.answerId).length;
  useEffect(() => {
    if (answered === questions.length && questions.length > 0) onComplete?.(score);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [answered, questions.length]);

  return (
    <div className={cn("flex flex-col gap-4", className)}>
      {questions.map((q) => {
        const picked = answers[q.id];
        return (
          <Card key={q.id}>
            <div className="mb-2 text-sm text-ink">{q.prompt}</div>
            <div className="flex flex-col gap-1.5">
              {q.choices.map((c) => {
                const isPicked = picked === c.id;
                const isAnswer = c.id === q.answerId;
                const tone = !picked
                  ? "border-line hover:bg-raise"
                  : isAnswer
                    ? "border-line text-for"
                    : isPicked
                      ? "border-line text-against"
                      : "border-line text-ink-3";
                return (
                  <button
                    key={c.id}
                    type="button"
                    disabled={!!picked}
                    onClick={() => setAnswers((prev) => ({ ...prev, [q.id]: c.id }))}
                    className={cn("rounded-chip border px-2.5 py-1.5 text-left text-sm transition-colors", tone)}
                  >
                    {c.text}
                  </button>
                );
              })}
            </div>
            {picked && q.explanation && <div className="mt-2 text-[12px] text-ink-2">{q.explanation}</div>}
          </Card>
        );
      })}
      <div className="font-mono text-[11px] text-ink-3">
        {answered}/{questions.length} answered · {score} correct
      </div>
    </div>
  );
}

/** Flashcards — flip and step through a deck; remembers position. */
export function Flashcards({
  cards,
  stateKey,
  className,
}: {
  cards: Array<{ id: string; front: ReactNode; back: ReactNode }>;
  stateKey?: string;
  className?: string;
}) {
  const [index, setIndex] = usePersisted<number>(stateKey, 0);
  const [flipped, setFlipped] = useState(false);
  if (cards.length === 0) return <EmptyState title="No cards" />;
  const card = cards[Math.min(index, cards.length - 1)];
  const step = (delta: number) => {
    setFlipped(false);
    setIndex((prev) => Math.max(0, Math.min(cards.length - 1, prev + delta)));
  };
  return (
    <div className={cn("flex flex-col items-center gap-3", className)}>
      <button
        type="button"
        onClick={() => setFlipped((f) => !f)}
        className="min-h-32 w-full rounded-card border border-line bg-card p-6 text-center text-ink transition-colors hover:bg-raise"
      >
        {flipped ? card.back : card.front}
      </button>
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="sm" onClick={() => step(-1)} disabled={index === 0}>
          ‹ Prev
        </Button>
        <span className="font-mono text-[11px] text-ink-3">
          {Math.min(index, cards.length - 1) + 1} / {cards.length}
        </span>
        <Button variant="ghost" size="sm" onClick={() => step(1)} disabled={index >= cards.length - 1}>
          Next ›
        </Button>
      </div>
    </div>
  );
}

/**
 * Timer — a countdown that survives reload: it persists the TARGET timestamp,
 * not a tick count, so a running timer resumes at the right remaining time.
 */
export function Timer({
  seconds,
  label,
  stateKey,
  onComplete,
  className,
}: {
  seconds: number;
  label?: ReactNode;
  stateKey?: string;
  onComplete?: () => void;
  className?: string;
}) {
  const [endsAt, setEndsAt] = usePersisted<number | null>(stateKey, null);
  const [remaining, setRemaining] = useState(seconds);
  const fired = useState(() => ({ done: false }))[0];

  useEffect(() => {
    if (!endsAt) {
      setRemaining(seconds);
      fired.done = false;
      return;
    }
    const tick = () => {
      const left = Math.max(0, Math.ceil((endsAt - Date.now()) / 1000));
      setRemaining(left);
      if (left === 0 && !fired.done) {
        fired.done = true;
        onComplete?.();
      }
    };
    tick();
    const id = setInterval(tick, 250);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [endsAt, seconds]);

  const mm = String(Math.floor(remaining / 60)).padStart(2, "0");
  const ss = String(remaining % 60).padStart(2, "0");
  const running = !!endsAt && remaining > 0;
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <div className="font-display text-2xl tabular-nums text-ink">
        {mm}:{ss}
      </div>
      {label && <span className="text-sm text-ink-2">{label}</span>}
      <Button size="sm" variant={running ? "outline" : "primary"} onClick={() => setEndsAt(running ? null : Date.now() + seconds * 1000)}>
        {running ? "Stop" : "Start"}
      </Button>
    </div>
  );
}

/** Checklist — persistent ticks. */
export function Checklist({
  items,
  stateKey,
  onChange,
  className,
}: {
  items: Array<{ id: string; label: ReactNode }>;
  stateKey?: string;
  onChange?: (checked: Record<string, boolean>) => void;
  className?: string;
}) {
  const [checked, setChecked] = usePersisted<Record<string, boolean>>(stateKey, {});
  const done = items.filter((i) => checked[i.id]).length;
  return (
    <div className={cn("flex flex-col gap-2", className)}>
      {items.map((item) => (
        <Checkbox
          key={item.id}
          checked={!!checked[item.id]}
          label={<span className={cn(checked[item.id] && "text-ink-3 line-through")}>{item.label}</span>}
          onChange={(next) =>
            setChecked((prev) => {
              const updated = { ...prev, [item.id]: next };
              onChange?.(updated);
              return updated;
            })
          }
        />
      ))}
      <ProgressBar value={done} max={items.length || 1} label={`${done}/${items.length} done`} />
    </div>
  );
}

/** Stepper — a linear walkthrough that remembers where you were. */
export function Stepper({
  steps,
  stateKey,
  className,
}: {
  steps: Array<{ id: string; title: ReactNode; content?: ReactNode }>;
  stateKey?: string;
  className?: string;
}) {
  const [current, setCurrent] = usePersisted<number>(stateKey, 0);
  if (steps.length === 0) return <EmptyState title="No steps" />;
  const index = Math.max(0, Math.min(current, steps.length - 1));
  return (
    <div className={cn("flex flex-col gap-3", className)}>
      <ol className="flex flex-wrap items-center gap-2">
        {steps.map((s, i) => (
          <li key={s.id} className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => setCurrent(i)}
              className={cn(
                "flex h-6 w-6 items-center justify-center rounded-full border font-mono text-[11px] transition-colors",
                i === index
                  ? "border-amber-line bg-amber-soft text-amber"
                  : i < index
                    ? "border-line text-for"
                    : "border-line text-ink-3 hover:text-ink",
              )}
              aria-current={i === index ? "step" : undefined}
            >
              {i + 1}
            </button>
            <span className={cn("text-sm", i === index ? "text-ink" : "text-ink-3")}>{s.title}</span>
            {i < steps.length - 1 && <span className="text-ink-4">›</span>}
          </li>
        ))}
      </ol>
      <Card>{steps[index].content}</Card>
      <div className="flex gap-2">
        <Button variant="ghost" size="sm" onClick={() => setCurrent(index - 1)} disabled={index === 0}>
          ‹ Back
        </Button>
        <Button size="sm" onClick={() => setCurrent(index + 1)} disabled={index >= steps.length - 1}>
          Next ›
        </Button>
      </div>
    </div>
  );
}

// --- L5 bridge (host ↔ shard channel: nodes.get / nodes.subscribe) -----------
//
// A shard is sandboxed (opaque origin); it reaches workspace/graph data only
// through this postMessage bridge. The SHARD code is identical in preview and
// production — it posts to window.parent; in the live app the HOST answers (auth +
// data, scoped to the shard's manifest refs), and in the headless preview a small
// emulator answers with stub nodes so data-wired shards still render. Messages are
// namespaced { aladin: "bridge/1" }; everything else is ignored.

const BRIDGE = "bridge/1";

// Node is a workspace/graph entity a shard depends on (declared in anchors.json
// `refs`). Shape is intentionally generic; `data` carries the entity payload.
export type Node = { id: string; type?: string; title?: string; data?: unknown };

let _seq = 0;
const _pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
const _subs = new Map<string, (n: Node) => void>();
let _wired = false;

// --- theme (host-pushed) ------------------------------------------------------
// The serve route stamps <html data-theme> for a correct first paint; the host
// then pushes {channel:"theme"} on every switch. Stamping the attribute is the
// whole activation — theme.css ships every [data-theme] block, so utilities and
// var() chains flip with zero re-render. _themeSubs exists for code that
// resolves tokens to concrete values at render (tok(), chart helpers): useTheme
// re-renders those consumers so they re-read computed styles.
let _theme = "";
const _themeSubs = new Set<() => void>();

function applyTheme(theme: unknown) {
  if (typeof theme !== "string" || theme === "" || theme === _theme) return;
  _theme = theme;
  document.documentElement.dataset.theme = theme;
  _themeSubs.forEach((fn) => fn());
}

// BridgeError carries the host's structured failure: `code` discriminates
// ("conflict" | "quota" | "too-large" | "unknown-method" | …) and `data` holds
// the code-specific payload (a conflict's { currentRevision, currentValue }).
export class BridgeError extends Error {
  code: string;
  data: unknown;
  constructor(message: string, code: string, data: unknown) {
    super(message);
    this.name = "BridgeError";
    this.code = code;
    this.data = data;
  }
}

function ensureWired() {
  if (_wired || typeof window === "undefined") return;
  _wired = true;
  window.addEventListener("message", (e: MessageEvent) => {
    const m = e.data as { aladin?: string; type?: string; id?: number; ok?: boolean; data?: unknown; error?: string; code?: string; channel?: string };
    if (!m || m.aladin !== BRIDGE) return;
    if (m.type === "response" && m.id != null && _pending.has(m.id)) {
      const p = _pending.get(m.id)!;
      _pending.delete(m.id);
      if (m.ok) p.resolve(m.data);
      else p.reject(new BridgeError(m.error || "bridge error", m.code || "error", m.data));
    } else if (m.type === "push" && m.channel === "theme") {
      applyTheme((m.data as { theme?: string } | null)?.theme);
    } else if (m.type === "push" && m.channel && _subs.has(m.channel)) {
      _subs.get(m.channel)!(m.data as Node);
    }
  });
  // Seed from the served document, then reconcile with the host (covers a theme
  // switch that happened while this frame was hidden in the keep-alive set, and
  // hosts that serve no stamp). Fire-and-forget: previews without a theme-aware
  // emulator just keep the stamp.
  _theme = document.documentElement.dataset.theme || "";
  post("theme.get", {})
    .then((d) => applyTheme((d as { theme?: string } | null)?.theme))
    .catch(() => {});
}

function post(method: string, params: Record<string, unknown>): Promise<unknown> {
  if (typeof window === "undefined") return Promise.reject(new Error("bridge: no window"));
  ensureWired();
  const id = ++_seq;
  return new Promise((resolve, reject) => {
    _pending.set(id, { resolve, reject });
    (window.parent || window).postMessage({ aladin: BRIDGE, type: "request", id, method, params }, "*");
    setTimeout(() => {
      if (_pending.has(id)) {
        _pending.delete(id);
        reject(new Error("bridge: timeout on " + method));
      }
    }, 8000);
  });
}

// bridge is the low-level client. Most shards use the useNode/useNodes hooks.
export const bridge = {
  getNodes(ids: string[]): Promise<Node[]> {
    return post("nodes.get", { ids }).then((d) => (d as Node[]) || []);
  },
  getNode(id: string): Promise<Node | null> {
    return post("nodes.get", { ids: [id] }).then((d) => ((d as Node[]) || [])[0] ?? null);
  },
  // subscribe pushes the current value then updates; returns an unsubscribe fn.
  subscribe(ids: string[], cb: (n: Node) => void): () => void {
    ensureWired();
    const channel = "sub:" + ++_seq;
    _subs.set(channel, cb);
    post("nodes.subscribe", { ids, channel }).catch(() => {});
    return () => {
      _subs.delete(channel);
      post("nodes.unsubscribe", { channel }).catch(() => {});
    };
  },
};

// useTheme returns the active Aladin theme name ("dark", "light", …) and
// re-renders on host theme switches. Utilities and var()-based styles follow the
// theme with NO code — reach for this hook only when a value is resolved at
// render time (tok(), chartSeries(), hand-computed colors).
export function useTheme(): string {
  return useSyncExternalStore(
    (onChange) => {
      _themeSubs.add(onChange);
      return () => _themeSubs.delete(onChange);
    },
    () => _theme,
    () => _theme,
  );
}

// Wire at import so every kit-using shard receives theme pushes immediately —
// not only after its first bridge call.
ensureWired();

// --- shard local state (kv) ---------------------------------------------------
//
// The shard's private key/value document store (design/SHARD_LOCAL_STATE.md),
// served by the host bridge: path-shaped keys, per-key revisions, prefix
// subscriptions. The host owns channel selection (the app binds your real data;
// previews get a scratch sandbox) — shard code never sees channels. Most shards
// use the hooks; `kv` is the imperative client.

export type KVEntry = { key: string; value: unknown; revision: number; deleted?: boolean };

export const kv = {
  get(key: string): Promise<KVEntry | null> {
    return post("kv.get", { key }).then((d) => (d as KVEntry) ?? null);
  },
  list(prefix = ""): Promise<KVEntry[]> {
    return post("kv.list", { prefix }).then((d) => ((d as { entries?: KVEntry[] })?.entries ?? []));
  },
  set(key: string, value: unknown, baseRevision: number): Promise<{ revision: number }> {
    return post("kv.set", { key, value, baseRevision }) as Promise<{ revision: number }>;
  },
  remove(key: string, baseRevision: number): Promise<void> {
    return post("kv.delete", { key, baseRevision }).then(() => undefined);
  },
  // subscribe pushes the current entries under prefix, then every change
  // (deleted:true for tombstones); returns an unsubscribe fn.
  subscribe(prefix: string, cb: (entry: KVEntry) => void): () => void {
    ensureWired();
    const channel = "kv:" + ++_seq;
    _subs.set(channel, cb as unknown as (n: Node) => void);
    post("kv.subscribe", { prefix, channel }).catch(() => {});
    return () => {
      _subs.delete(channel);
      post("kv.unsubscribe", { channel }).catch(() => {});
    };
  },
};

// conflictOf narrows a rejection to the conflict payload, or null.
function conflictOf(err: unknown): { currentRevision: number; currentValue: unknown } | null {
  if (err instanceof BridgeError && err.code === "conflict") {
    const d = (err.data ?? {}) as { currentRevision?: number; currentValue?: unknown };
    return { currentRevision: d.currentRevision ?? 0, currentValue: d.currentValue };
  }
  return null;
}

/**
 * useShardState — persistent widget state under one key. Renders instantly from
 * local state (the shard is the single writer of its own view); persists
 * write-through with the per-key revision guard, and on a conflict (another
 * client edited the same key) re-applies your updater to the stored current and
 * retries — generated code never handles concurrency by hand. Live pushes from
 * other clients adopt automatically (revision-guarded).
 */
export function useShardState<T>(
  key: string,
  initial: T,
): [T, (next: T | ((prev: T) => T)) => void, { loading: boolean; error: string | null }] {
  const [value, setValue] = useState<T>(initial);
  const [meta, setMeta] = useState<{ loading: boolean; error: string | null }>({ loading: true, error: null });
  const stateRef = useState(() => ({ revision: 0, value: initial, alive: true, chain: Promise.resolve() }))[0];

  useEffect(() => {
    stateRef.alive = true;
    kv.get(key)
      .then((entry) => {
        if (!stateRef.alive) return;
        if (entry) {
          stateRef.revision = entry.revision;
          stateRef.value = entry.value as T;
          setValue(entry.value as T);
        }
        setMeta({ loading: false, error: null });
      })
      .catch((e: Error) => {
        if (stateRef.alive) setMeta({ loading: false, error: e.message });
      });
    const unsub = kv.subscribe(key, (entry) => {
      if (!stateRef.alive || entry.key !== key) return;
      if (entry.revision <= stateRef.revision) return; // echo / stale
      stateRef.revision = entry.revision;
      if (!entry.deleted) {
        stateRef.value = entry.value as T;
        setValue(entry.value as T);
      }
    });
    return () => {
      stateRef.alive = false;
      unsub();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const set = (next: T | ((prev: T) => T)) => {
    const updater = typeof next === "function" ? (next as (prev: T) => T) : null;
    const desired = updater ? updater(stateRef.value) : (next as T);
    stateRef.value = desired;
    setValue(desired);
    // Serialize persists; on conflict re-apply the updater to the stored
    // current (bounded retries), else last-write-wins with the fresh revision.
    stateRef.chain = stateRef.chain.then(async () => {
      let attempt = 0;
      let target = stateRef.value;
      for (;;) {
        try {
          const res = await kv.set(key, target, stateRef.revision);
          stateRef.revision = res.revision;
          if (stateRef.alive) setMeta({ loading: false, error: null });
          return;
        } catch (err) {
          const conflict = conflictOf(err);
          if (!conflict || attempt >= 3) {
            if (stateRef.alive) setMeta({ loading: false, error: err instanceof Error ? err.message : String(err) });
            return;
          }
          attempt++;
          stateRef.revision = conflict.currentRevision;
          target = updater ? updater(conflict.currentValue as T) : target;
          stateRef.value = target;
          if (stateRef.alive) setValue(target);
        }
      }
    });
  };

  return [value, set, meta];
}

/**
 * useKV — a live view of every key under a prefix (the shard's mini-app data:
 * "expenses/", "annotations/"…). put/remove are revision-guarded internally; a
 * put that loses a race retries once against the stored revision (the user's
 * whole-document action wins).
 */
export function useKV(prefix: string): {
  entries: Record<string, unknown>;
  put(key: string, value: unknown): void;
  remove(key: string): void;
  loading: boolean;
  error: string | null;
} {
  const [entries, setEntries] = useState<Record<string, unknown>>({});
  const [meta, setMeta] = useState<{ loading: boolean; error: string | null }>({ loading: true, error: null });
  const revsRef = useState(() => new Map<string, number>())[0];

  useEffect(() => {
    setEntries({});
    revsRef.clear();
    setMeta({ loading: true, error: null });
    const unsub = kv.subscribe(prefix, (entry) => {
      const known = revsRef.get(entry.key) ?? 0;
      if (entry.revision <= known) return;
      revsRef.set(entry.key, entry.revision);
      setEntries((prev) => {
        const next = { ...prev };
        if (entry.deleted) delete next[entry.key];
        else next[entry.key] = entry.value;
        return next;
      });
      setMeta({ loading: false, error: null });
    });
    // Subscribe seeds current entries; an empty prefix set still ends loading.
    kv.list(prefix)
      .then(() => setMeta((m) => ({ ...m, loading: false })))
      .catch((e: Error) => setMeta({ loading: false, error: e.message }));
    return unsub;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [prefix]);

  const write = (key: string, value: unknown, isDelete: boolean) => {
    const attempt = (baseRevision: number, retried: boolean) => {
      const op = isDelete ? kv.remove(key, baseRevision) : kv.set(key, value, baseRevision).then(() => undefined);
      op.catch((err: unknown) => {
        const conflict = conflictOf(err);
        if (conflict && !retried) {
          revsRef.set(key, conflict.currentRevision);
          attempt(conflict.currentRevision, true);
          return;
        }
        setMeta({ loading: false, error: err instanceof Error ? err.message : String(err) });
      });
    };
    attempt(revsRef.get(key) ?? 0, false);
  };

  return {
    entries,
    put: (key, value) => write(key, value, false),
    remove: (key) => write(key, undefined, true),
    loading: meta.loading,
    error: meta.error,
  };
}

export type NodeState = { node: Node | null; loading: boolean; error: string | null };

// useNode fetches a single node and live-updates it via subscription. id may be
// null/undefined to render nothing. Use the returned {node, loading, error}.
export function useNode(id: string | null | undefined): NodeState {
  const [state, setState] = useState<NodeState>({ node: null, loading: !!id, error: null });
  useEffect(() => {
    if (!id) {
      setState({ node: null, loading: false, error: null });
      return;
    }
    let alive = true;
    setState({ node: null, loading: true, error: null });
    bridge
      .getNode(id)
      .then((n) => {
        if (alive) setState({ node: n, loading: false, error: null });
      })
      .catch((e: Error) => {
        if (alive) setState({ node: null, loading: false, error: e.message });
      });
    const unsub = bridge.subscribe([id], (n) => {
      if (alive && n && n.id === id) setState({ node: n, loading: false, error: null });
    });
    return () => {
      alive = false;
      unsub();
    };
  }, [id]);
  return state;
}

// useNodes is the multi-id form: returns {nodes, loading, error} with nodes keyed
// in the requested order (missing ids omitted).
export function useNodes(ids: string[]): { nodes: Node[]; loading: boolean; error: string | null } {
  const key = ids.join(",");
  const [state, setState] = useState<{ nodes: Node[]; loading: boolean; error: string | null }>({
    nodes: [],
    loading: ids.length > 0,
    error: null,
  });
  useEffect(() => {
    if (ids.length === 0) {
      setState({ nodes: [], loading: false, error: null });
      return;
    }
    let alive = true;
    setState({ nodes: [], loading: true, error: null });
    bridge
      .getNodes(ids)
      .then((ns) => {
        if (alive) setState({ nodes: ns, loading: false, error: null });
      })
      .catch((e: Error) => {
        if (alive) setState({ nodes: [], loading: false, error: e.message });
      });
    const unsub = bridge.subscribe(ids, (n) => {
      if (!alive) return;
      setState((s) => ({ ...s, loading: false, nodes: s.nodes.map((x) => (x.id === n.id ? n : x)) }));
    });
    return () => {
      alive = false;
      unsub();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  return state;
}
