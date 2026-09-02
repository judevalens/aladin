import { useMemo, useState } from "react";
import type { ButtonHTMLAttributes, ReactNode, TextareaHTMLAttributes } from "react";
import { createRoot } from "react-dom/client";
import { useShardState } from "@aladin/shard";

type Tone = "neutral" | "amber" | "for" | "against";

const toneClasses: Record<Tone, string> = {
  neutral: "border-line bg-raise text-ink-2",
  amber: "border-amber-line bg-amber-soft text-amber",
  for: "border-for/30 bg-for/10 text-for",
  against: "border-against/30 bg-against/10 text-against",
};

function Page({ children }: { children: ReactNode }) {
  return <main className="min-h-screen bg-bg text-ink">{children}</main>;
}

function Section({ children }: { children: ReactNode }) {
  return <div className="mx-auto w-full max-w-6xl px-5 py-8 sm:px-8">{children}</div>;
}

function Region({ anchor, kind, children }: { anchor: string; kind: string; children: ReactNode }) {
  return <section data-anchor={anchor} data-kind={kind}>{children}</section>;
}

function Badge({ tone = "neutral", children }: { tone?: Tone; children: ReactNode }) {
  return <span className={`inline-flex items-center rounded-chip border px-2 py-1 font-mono text-meta uppercase tracking-wider ${toneClasses[tone]}`}>{children}</span>;
}

function Button({ variant = "outline", size = "md", className = "", ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "outline" | "ghost"; size?: "sm" | "md" }) {
  const variants = {
    primary: "border-amber bg-amber text-bg hover:opacity-90",
    outline: "border-line bg-panel text-ink hover:border-line-2 hover:bg-raise",
    ghost: "border-transparent bg-transparent text-ink-2 hover:bg-raise hover:text-ink",
  };
  return <button type="button" className={`rounded-control border font-sans transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-amber ${size === "sm" ? "px-3 py-1.5 text-small" : "px-4 py-2 text-body"} ${variants[variant]} ${className}`} {...props} />;
}

function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <article className={`rounded-card border border-line bg-panel p-4 shadow-panel ${className}`}>{children}</article>;
}

function Panel({ children }: { children: ReactNode }) {
  return <div className="rounded-card border border-line bg-field p-4">{children}</div>;
}

function Divider() {
  return <hr className="border-0 border-t border-line" />;
}

function Textarea({ className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={`w-full resize-y rounded-control border border-line bg-field px-3 py-2 text-body text-ink outline-none transition-colors placeholder:text-ink-3 focus:border-amber ${className}`} {...props} />;
}

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <div className="rounded-card border border-line bg-panel p-4">
      <div className="font-mono text-meta uppercase tracking-wider text-ink-3">{label}</div>
      <div className="mt-2 font-display text-title text-ink">{value}</div>
      <div className="mt-1 text-small text-ink-2">{sub}</div>
    </div>
  );
}

function ProgressBar({ value, max, label }: { value: number; max: number; label: string }) {
  const percent = max > 0 ? Math.round((value / max) * 100) : 0;
  return (
    <div>
      <div className="mb-2 flex justify-between font-mono text-meta uppercase tracking-wider text-ink-3"><span>Progress</span><span>{label}</span></div>
      <div className="h-2 overflow-hidden rounded-full bg-field"><div className="h-full rounded-full bg-amber transition-[width]" style={{ width: `${percent}%` }} /></div>
    </div>
  );
}

function Callout({ title, children }: { tone?: "warn"; title: string; children: ReactNode }) {
  return (
    <aside className="rounded-card border border-amber-line bg-amber-soft p-4 text-ink-2">
      <h3 className="font-display text-body text-amber">{title}</h3>
      <div className="mt-2 text-small leading-relaxed">{children}</div>
    </aside>
  );
}

type PhaseId = "stabilize" | "map" | "consolidate" | "rewrite" | "harden";
type Area = "program" | "product" | "frontend" | "backend" | "shards" | "verification";
type Status = "planned" | "ready" | "active" | "blocked" | "done";
type Priority = "critical" | "high" | "medium";

type Phase = {
  id: PhaseId;
  order: number;
  title: string;
  purpose: string;
  gate: string;
};

type WorkItem = {
  id: string;
  phase: PhaseId;
  area: Area;
  priority: Priority;
  title: string;
  outcome: string;
};

type PlannerState = {
  currentFocus: string;
  decisionNote: string;
  statuses: Record<string, Status>;
  updatedAt: string;
};

const PHASES: Phase[] = [
  {
    id: "stabilize",
    order: 0,
    title: "Stabilize the frontier",
    purpose: "Checkpoint active Shard v2 and board work before structural movement.",
    gate: "The current frontier can be resumed or reverted without reconstructing intent.",
  },
  {
    id: "map",
    order: 1,
    title: "Establish the map",
    purpose: "Assign ownership, dependencies, invariants, lifecycle, and product status.",
    gate: "Every major behavior has one intended owner or one explicit open decision.",
  },
  {
    id: "consolidate",
    order: 2,
    title: "Consolidate boundaries",
    purpose: "Reduce conceptual surface area and make dependency direction enforceable.",
    gate: "New work has an obvious home and parked systems no longer shape active architecture.",
  },
  {
    id: "rewrite",
    order: 3,
    title: "Targeted rewrites",
    purpose: "Replace only modules whose current pattern conflicts with stable boundaries.",
    gate: "Each replacement has a smaller public surface and no parallel legacy path.",
  },
  {
    id: "harden",
    order: 4,
    title: "Quality hardening",
    purpose: "Improve naming, failure semantics, tests, observability, and local clarity.",
    gate: "Normal verification maintains quality without periodic heroic cleanup.",
  },
];

const WORK: WorkItem[] = [
  { id: "frontier-checkpoint", phase: "stabilize", area: "program", priority: "critical", title: "Checkpoint the active frontier", outcome: "Named checkpoint, test baseline, and explicit resume or rollback boundary." },
  { id: "working-tree-inventory", phase: "stabilize", area: "program", priority: "high", title: "Inventory active uncommitted work", outcome: "Each change belongs to a known feature, protocol, migration, or follow-up." },
  { id: "module-inventory", phase: "map", area: "program", priority: "critical", title: "Generate the module inventory", outcome: "Current responsibility, owner, API, dependencies, state, invariants, and tests." },
  { id: "product-classification", phase: "map", area: "product", priority: "critical", title: "Classify every product surface", outcome: "Active, supporting, transitional, parked, or historical status is explicit." },
  { id: "frontend-map", phase: "map", area: "frontend", priority: "high", title: "Map frontend dependency and state ownership", outcome: "Rendering, orchestration, host integration, and persistence boundaries are visible." },
  { id: "backend-map", phase: "map", area: "backend", priority: "high", title: "Map backend dependency and transaction ownership", outcome: "API, service, repository, MCP, outbox, and protocol responsibilities are visible." },
  { id: "hotspot-register", phase: "map", area: "verification", priority: "high", title: "Create the hotspot register", outcome: "Size, churn, coupling, test reach, and failure history replace intuition alone." },
  { id: "parked-surface", phase: "consolidate", area: "product", priority: "high", title: "Quarantine one parked surface end to end", outcome: "Parked code stops influencing active navigation, state, and architecture." },
  { id: "duplicate-path", phase: "consolidate", area: "program", priority: "high", title: "Consolidate one duplicate access path", outcome: "One owner and one production path remain, with compatibility removal recorded." },
  { id: "dependency-rule", phase: "consolidate", area: "verification", priority: "medium", title: "Enforce one dependency rule", outcome: "A documented boundary becomes executable verification." },
  { id: "stable-extraction", phase: "consolidate", area: "backend", priority: "high", title: "Extract one stable subsystem", outcome: "Behavior and tests move together without changing product behavior." },
  { id: "rewrite-proposal", phase: "rewrite", area: "program", priority: "critical", title: "Approve the first rewrite proposal", outcome: "Failure mode, replacement pattern, migration, rollback, tests, and deletion are explicit." },
  { id: "rewrite-slice", phase: "rewrite", area: "program", priority: "high", title: "Execute one reversible rewrite slice", outcome: "The replacement ships with the superseded implementation removed." },
  { id: "architecture-checks", phase: "harden", area: "verification", priority: "high", title: "Add recurring architecture checks", outcome: "Dependency drift and compatibility residue become visible during normal verification." },
  { id: "quality-baseline", phase: "harden", area: "verification", priority: "medium", title: "Record the quality baseline", outcome: "Locality, test scope, duplicate paths, and module ownership can be compared over time." },
];

const INITIAL: PlannerState = {
  currentFocus: "Finish and checkpoint the active Shard v2 and board frontier.",
  decisionNote: "Architecture consolidation precedes any major module rewrite.",
  statuses: Object.fromEntries(WORK.map((item) => [item.id, item.phase === "stabilize" ? "ready" : "planned"])),
  updatedAt: "2026-08-31",
};

const STATUS_ORDER: Status[] = ["planned", "ready", "active", "blocked", "done"];

function statusTone(status: Status): "neutral" | "amber" | "for" | "against" {
  if (status === "done") return "for";
  if (status === "active" || status === "ready") return "amber";
  if (status === "blocked") return "against";
  return "neutral";
}

function label(value: string): string {
  return value.replaceAll("_", " ");
}

function App() {
  const [state, setState, persistence] = useShardState<PlannerState>("architecture-refactor/program-v1", INITIAL);
  const [phaseFilter, setPhaseFilter] = useState<PhaseId | "all">("all");
  const [statusFilter, setStatusFilter] = useState<Status | "open" | "all">("open");

  const statuses = { ...INITIAL.statuses, ...state.statuses };
  const done = WORK.filter((item) => statuses[item.id] === "done").length;
  const active = WORK.filter((item) => statuses[item.id] === "active").length;
  const blocked = WORK.filter((item) => statuses[item.id] === "blocked").length;
  const currentPhase = PHASES.find((phase) =>
    WORK.some((item) => item.phase === phase.id && statuses[item.id] !== "done"),
  ) ?? PHASES[PHASES.length - 1];

  const visible = useMemo(() => WORK.filter((item) => {
    const status = statuses[item.id];
    const phaseMatches = phaseFilter === "all" || item.phase === phaseFilter;
    const statusMatches = statusFilter === "all"
      || status === statusFilter
      || (statusFilter === "open" && status !== "done");
    return phaseMatches && statusMatches;
  }), [phaseFilter, statusFilter, statuses]);

  function updateStatus(id: string, status: Status) {
    setState((current) => ({
      ...current,
      statuses: { ...current.statuses, [id]: status },
      updatedAt: new Date().toISOString(),
    }));
  }

  function updateText(field: "currentFocus" | "decisionNote", value: string) {
    setState((current) => ({ ...current, [field]: value, updatedAt: new Date().toISOString() }));
  }

  return (
    <Page>
      <Section>
        <Region anchor="program-overview" kind="metric">
          <div className="flex flex-col gap-5 py-6">
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div className="max-w-2xl">
                <div className="mb-3 flex flex-wrap gap-2">
                  <Badge tone="amber">Architecture program</Badge>
                  <Badge tone={persistence.error ? "against" : "neutral"}>
                    {persistence.loading ? "Loading state" : persistence.error ? "Persistence issue" : "Persistent plan"}
                  </Badge>
                </div>
                <h1 className="font-display text-display text-ink">Consolidate first. Rewrite second.</h1>
                <p className="mt-3 text-lead text-ink-2">
                  Reduce conceptual surface area, stabilize ownership, then rewrite only the modules whose patterns still resist local change.
                </p>
              </div>
              <div className="font-mono text-small text-ink-3">Draft 0.1 · 2026-08-31</div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <Stat label="Program progress" value={`${done}/${WORK.length}`} sub="work items complete" />
              <Stat label="Current phase" value={`${currentPhase.order}`} sub={currentPhase.title} />
              <Stat label="Active" value={String(active)} sub="items in motion" />
              <Stat label="Blocked" value={String(blocked)} sub="items needing a decision" />
            </div>
            <ProgressBar value={done} max={WORK.length} label={`${Math.round((done / WORK.length) * 100)}% complete`} />

            <Panel>
              <div className="grid gap-4 md:grid-cols-2">
                <label className="block">
                  <span className="mb-2 block text-meta uppercase tracking-wider text-ink-3">Current focus</span>
                  <Textarea value={state.currentFocus} onChange={(event) => updateText("currentFocus", event.target.value)} rows={3} />
                </label>
                <label className="block">
                  <span className="mb-2 block text-meta uppercase tracking-wider text-ink-3">Decision note</span>
                  <Textarea value={state.decisionNote} onChange={(event) => updateText("decisionNote", event.target.value)} rows={3} />
                </label>
              </div>
            </Panel>
          </div>
        </Region>

        <Divider />

        <Region anchor="program-phases" kind="collection">
          <div className="py-6">
            <div className="mb-4">
              <h2 className="font-display text-title text-ink">Program phases</h2>
              <p className="mt-1 text-body text-ink-2">Each gate prevents clean code from being built inside an unstable boundary.</p>
            </div>
            <div className="grid gap-3">
              {PHASES.map((phase) => {
                const phaseItems = WORK.filter((item) => item.phase === phase.id);
                const phaseDone = phaseItems.filter((item) => statuses[item.id] === "done").length;
                const isCurrent = currentPhase.id === phase.id;
                return (
                  <Card key={phase.id} className={isCurrent ? "border-amber-line" : ""}>
                    <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-chip bg-raise font-mono text-small text-ink">
                        {phase.order}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="font-display text-body text-ink">{phase.title}</h3>
                          {isCurrent && <Badge tone="amber">Current</Badge>}
                          <Badge tone={phaseDone === phaseItems.length ? "for" : "neutral"}>{phaseDone}/{phaseItems.length}</Badge>
                        </div>
                        <p className="mt-2 text-body text-ink-2">{phase.purpose}</p>
                        <p className="mt-2 text-small text-ink-3"><span className="font-mono text-ink-2">EXIT</span> · {phase.gate}</p>
                      </div>
                    </div>
                  </Card>
                );
              })}
            </div>
          </div>
        </Region>

        <Divider />

        <Region anchor="program-backlog" kind="collection">
          <div className="py-6">
            <div className="flex flex-wrap items-end justify-between gap-4">
              <div>
                <h2 className="font-display text-title text-ink">Execution backlog</h2>
                <p className="mt-1 text-body text-ink-2">The document defines the policy; this board records movement.</p>
              </div>
              <div className="text-small text-ink-3">{visible.length} visible</div>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              {(["all", ...PHASES.map((phase) => phase.id)] as const).map((phase) => (
                <Button key={phase} size="sm" variant={phaseFilter === phase ? "primary" : "outline"} onClick={() => setPhaseFilter(phase)}>
                  {phase === "all" ? "All phases" : PHASES.find((entry) => entry.id === phase)?.title}
                </Button>
              ))}
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              {(["open", "all", ...STATUS_ORDER] as const).map((status) => (
                <Button key={status} size="sm" variant={statusFilter === status ? "primary" : "ghost"} onClick={() => setStatusFilter(status)}>
                  {label(status)}
                </Button>
              ))}
            </div>

            <div className="mt-5 grid gap-3">
              {visible.map((item) => {
                const status = statuses[item.id];
                return (
                  <Card key={item.id}>
                    <div className="flex flex-col gap-4 lg:flex-row lg:items-center">
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge tone={statusTone(status)}>{label(status)}</Badge>
                          <Badge tone="neutral">{item.area}</Badge>
                          <span className="font-mono text-meta uppercase text-ink-3">{item.priority}</span>
                        </div>
                        <h3 className="mt-2 font-display text-body text-ink">{item.title}</h3>
                        <p className="mt-1 text-small text-ink-2">{item.outcome}</p>
                      </div>
                      <label className="shrink-0">
                        <span className="sr-only">Status for {item.title}</span>
                        <select
                          className="rounded-control border border-line bg-field px-3 py-2 text-small text-ink"
                          value={status}
                          onChange={(event) => updateStatus(item.id, event.target.value as Status)}
                        >
                          {STATUS_ORDER.map((option) => <option key={option} value={option}>{label(option)}</option>)}
                        </select>
                      </label>
                    </div>
                  </Card>
                );
              })}
            </div>
          </div>
        </Region>

        <Divider />

        <Region anchor="program-rewrite-gate" kind="control">
          <div className="py-6">
            <Callout tone="warn" title="Rewrite gate">
              A module is not rewritten because it is large, old, AI-assisted, or unfamiliar. A proposal must identify the current failure mode, stable responsibility, replacement pattern, migration sequence, rollback path, equivalence tests, and code that will be deleted.
            </Callout>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <Card>
                <h3 className="font-display text-body text-ink">Evidence that supports a rewrite</h3>
                <ul className="mt-3 space-y-2 text-small text-ink-2">
                  <li>Multiple unrelated reasons to change</li>
                  <li>Ambiguous or cyclic dependency direction</li>
                  <li>State ownership split across layers</li>
                  <li>Compatibility code dominating the active path</li>
                  <li>Extraction preserving the same confusion under more files</li>
                </ul>
              </Card>
              <Card>
                <h3 className="font-display text-body text-ink">Definition of a successful slice</h3>
                <ul className="mt-3 space-y-2 text-small text-ink-2">
                  <li>Behavior remains covered and observable</li>
                  <li>One owner and dependency direction are explicit</li>
                  <li>The superseded production path is removed</li>
                  <li>Operational behavior remains intact</li>
                  <li>The next change requires less system-wide context</li>
                </ul>
              </Card>
            </div>
          </div>
        </Region>
      </Section>
    </Page>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
