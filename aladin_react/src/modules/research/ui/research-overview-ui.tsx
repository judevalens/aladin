import { useEffect, useState } from "react";
import {
  Code2,
  FileText,
  Folder,
  Layout,
  Link2,
  Mic,
  Paperclip,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useAppStore } from "@/app/state/store";
import { useResearchOverview, type ResearchMaterial } from "@/modules/research/hooks/use-research-overview";
import type { ResearchView } from "@/modules/workspace/domain";

/**
 * The Overview — the front page of every research folder, and the first thing an agent
 * reads (RESEARCH_SURFACE_PRD §15).
 *
 * It's a TYPED artifact with fixed sections, not a page: a page can be emptied, and these
 * sections are live rather than authored.
 *
 * What it shows is governed by what's TRUE today. There is no engine yet (§22), so this
 * surface does not pretend: no disabled Launch button, no zeroed metric tiles, no empty
 * chart frames. §2 is explicit — no data-slop, no 40-widget dashboards. Until the engine
 * lands, a research folder is where the thinking about one strategy accumulates, so the
 * Overview answers the questions that actually have answers:
 *
 *   what do I believe · what have I gathered · what code is here
 *
 * The control-plane half (launch, arm/offline, live numbers, aggregate stats) arrives
 * with T2 and slots into the sections below it.
 */
export function ResearchOverviewUI({
  contextId,
  onOpenView,
}: {
  contextId: string;
  onOpenView: (view: ResearchView) => void;
}) {
  const { folder, material, loading, error, saving, onSaveHypothesis } =
    useResearchOverview(contextId);
  const openArtifact = useAppStore((state) => state.openArtifact);

  if (loading && !folder) {
    return (
      <div className="p-8 font-mono text-[11px] uppercase tracking-wide text-ink-4">Loading…</div>
    );
  }
  if (error) return <div className="p-8 text-[13px] text-against">{error}</div>;
  if (!folder) return null;

  return (
    <div className="mx-auto w-full max-w-workspace-max px-8 py-7">
      <header className="mb-6">
        <h1 className="font-display text-[24px] leading-tight text-ink">{folder.title}</h1>
        <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-[10.5px] uppercase tracking-[0.5px] text-ink-4">
          <span>{folder.execMode}-driven</span>
          <span>{material.length} item{material.length === 1 ? "" : "s"}</span>
          {folder.runState !== "idle" ? (
            <span className="text-amber">{folder.runState}</span>
          ) : null}
        </div>
      </header>

      <Hypothesis hypothesis={folder.hypothesis} saving={saving} onSave={onSaveHypothesis} />

      <Material items={material} onOpen={openArtifact} />

      <StrategyCode folder={folder} onOpenCode={() => onOpenView("code")} />
    </div>
  );
}

/**
 * §15's freeform region, and the reason the folder exists. Deliberately the most
 * prominent thing on the surface: TRADING_PRD §2 puts the hypothesis on the strategy
 * itself, and until there are runs it is the only claim this folder makes.
 *
 * The placeholder asks for the falsifier too — §4 bans an auto-optimizer for the same
 * reason this asks the question: research that can't be wrong isn't research.
 */
function Hypothesis({
  hypothesis,
  saving,
  onSave,
}: {
  hypothesis: string;
  saving: boolean;
  onSave: (value: string) => Promise<void>;
}) {
  const [draft, setDraft] = useState(hypothesis);
  useEffect(() => setDraft(hypothesis), [hypothesis]);

  return (
    <section className="mb-7">
      <SectionLabel>
        Hypothesis
        {saving ? <span className="ml-2 normal-case text-ink-4">saving…</span> : null}
      </SectionLabel>
      <textarea
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => {
          if (draft !== hypothesis) void onSave(draft);
        }}
        rows={3}
        placeholder="What do you believe — and what would show it's wrong?"
        className="w-full resize-y rounded-card border border-line bg-card px-4 py-3.5 font-display text-[15px] leading-relaxed text-ink outline-none transition-colors placeholder:font-sans placeholder:text-[13.5px] placeholder:text-ink-4 focus:border-amber-line"
      />
    </section>
  );
}

/**
 * What's actually in the folder (§3 #8, §21): papers, notes, links, voice memos, nested
 * folders. This is the half of the product that works today — a research folder is a
 * place material accumulates — and it was invisible on the Overview before.
 *
 * Reads straight off the workspace tree the syncer already maintains, so it needs no
 * fetch of its own and stays live as things are added.
 */
function Material({ items, onOpen }: { items: ResearchMaterial[]; onOpen: (id: string) => void }) {
  return (
    <section className="mb-7">
      <SectionLabel>Material · {items.length}</SectionLabel>
      {items.length === 0 ? (
        <p className="rounded-card border border-dashed border-line bg-card px-4 py-3.5 text-[13px] leading-relaxed text-ink-4">
          Nothing captured yet. Drop the paper, the forum thread, or the notes that made
          you think this was worth testing.
        </p>
      ) : (
        <ul className="overflow-hidden rounded-card border border-line">
          {items.map((item, index) => {
            const Icon = MATERIAL_ICONS[item.kind] ?? (item.isContainer ? Folder : FileText);
            return (
              <li key={item.id}>
                <button
                  type="button"
                  onClick={() => !item.isContainer && onOpen(item.id)}
                  disabled={item.isContainer}
                  className={cn(
                    "flex w-full items-center gap-3 bg-card px-3.5 py-2.5 text-left transition-colors",
                    index > 0 && "border-t border-line",
                    item.isContainer ? "cursor-default" : "hover:bg-raise",
                  )}
                >
                  <Icon className="size-4 shrink-0 text-ink-4" strokeWidth={1.75} />
                  <span className="min-w-0 flex-1 truncate text-[13.5px] text-ink-2">
                    {item.title || "Untitled"}
                  </span>
                  <span className="shrink-0 font-mono text-[10.5px] uppercase tracking-wide text-ink-4">
                    {item.kind}
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

/**
 * §7 (rev 5): a strategy is a DIRECTORY OF FILES, like any IDE project — never a
 * template, never a single document. So this reports where that directory came from and
 * what pins it, and says plainly when there isn't one.
 */
function StrategyCode({
  folder,
  onOpenCode,
}: {
  folder: { sourceKind: string; sourceRef?: string; commitSha?: string; codeHash?: string };
  onOpenCode: () => void;
}) {
  const attached = Boolean(folder.sourceRef);

  return (
    <section>
      <SectionLabel>Strategy code</SectionLabel>
      {!attached ? (
        <p className="rounded-card border border-dashed border-line bg-card px-4 py-3.5 text-[13px] leading-relaxed text-ink-4">
          No code yet. A strategy is a directory of files — import one from git, point at a
          local directory, or start writing here.
        </p>
      ) : (
        <button
          type="button"
          onClick={onOpenCode}
          className="flex w-full items-center gap-3 rounded-card border border-line bg-card px-3.5 py-3 text-left transition-colors hover:bg-raise"
        >
          <Code2 className="size-4 shrink-0 text-ink-4" strokeWidth={1.75} />
          <span className="min-w-0 flex-1">
            <span className="block truncate font-mono text-[12.5px] text-ink-2">
              {folder.sourceRef}
            </span>
            <span className="mt-0.5 block font-mono text-[10.5px] uppercase tracking-wide text-ink-4">
              {folder.sourceKind}
              {folder.commitSha ? ` · ${folder.commitSha.slice(0, 8)}` : ""}
              {folder.codeHash ? ` · ${folder.codeHash.slice(0, 8)}` : " · never run"}
            </span>
          </span>
        </button>
      )}
    </section>
  );
}

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <h2 className="mb-2 font-mono text-[10.5px] uppercase tracking-[0.6px] text-ink-4">
      {children}
    </h2>
  );
}

const MATERIAL_ICONS: Record<string, LucideIcon> = {
  note: FileText,
  page: FileText,
  link: Link2,
  voice: Mic,
  file: Paperclip,
  app: Layout,
  folder: Folder,
};
