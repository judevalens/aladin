import { ScrollArea } from "@/components/ui/scroll-area";
import { PlaceholderPane } from "@/components/ui/aladin";
import { useAppStore } from "@/app/state/store";
import { ResearchOverviewUI } from "@/modules/research/ui/research-overview-ui";
import { RESEARCH_VIEW_LABEL, type ResearchView } from "@/modules/workspace/domain";

/**
 * Routes a research tab to its view — the research arm of the work pane's kind switch
 * (RESEARCH_SURFACE_PRD §11). Each view is a branch here rather than a separate route,
 * so research reuses the existing workspace shell with no second tree and no rail entry.
 *
 * §22: Overview is buildable today; Control, Inspect and Runs wait on the engine, and
 * Code waits on a strategy directory existing. Those say so plainly instead of showing
 * an empty chrome that implies something is broken.
 */
export function ResearchPaneUI({ contextId, view }: { contextId: string; view: ResearchView }) {
  const openResearchTab = useAppStore((state) => state.openResearchTab);

  if (view === "overview") {
    return (
      <ScrollArea className="h-full">
        <ResearchOverviewUI
          contextId={contextId}
          onOpenView={(next) => openResearchTab(contextId, next)}
        />
      </ScrollArea>
    );
  }

  return (
    <PlaceholderPane
      title={RESEARCH_VIEW_LABEL[view]}
      body={PENDING_COPY[view]}
      className="h-full"
    />
  );
}

// Honest about WHY each surface is empty — §2 wants no onboarding, but a blank pane with
// no explanation is worse than one sentence of truth.
const PENDING_COPY: Record<Exclude<ResearchView, "overview">, string> = {
  manifest:
    "The manifest arrives with the strategy directory — imported from git, or a local directory you point at.",
  runs: "Every run is kept forever, immutable and citable. Nothing has run yet.",
  code: "A file tree beside an editor, like any IDE — a strategy is a directory of files, not a document. No code is attached to this folder yet.",
  inspect:
    "What the strategy saw on a bar — bars in, indicator values, the decision and its reason. Waits on the engine.",
};
