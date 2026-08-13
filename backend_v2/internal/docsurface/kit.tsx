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

function ensureWired() {
  if (_wired || typeof window === "undefined") return;
  _wired = true;
  window.addEventListener("message", (e: MessageEvent) => {
    const m = e.data as { aladin?: string; type?: string; id?: number; ok?: boolean; data?: unknown; error?: string; channel?: string };
    if (!m || m.aladin !== BRIDGE) return;
    if (m.type === "response" && m.id != null && _pending.has(m.id)) {
      const p = _pending.get(m.id)!;
      _pending.delete(m.id);
      if (m.ok) p.resolve(m.data);
      else p.reject(new Error(m.error || "bridge error"));
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
