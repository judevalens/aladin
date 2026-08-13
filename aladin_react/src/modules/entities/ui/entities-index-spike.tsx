import { Globe } from "lucide-react";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";

import { useAppStore } from "@/app/state/store";
import type { EntityListItem } from "@/modules/entities/entity-list-types";
import { EntityCard } from "@/modules/entities/ui/entity-card";

// Dev-only spike (/spike/entities-index) — the index cards + masonry on mock data,
// outside the login wall, so the design-matched card treatment can be checked in both
// themes. Not shipped.

const now = Date.now();
const ago = (days: number) => new Date(now - days * 86400_000).toISOString();

const MOCK: EntityListItem[] = [
  {
    id: "1", name: "counterintelligence", kind: "concept", trustTier: "believed",
    gist: "Whether AI agents embedded in intelligence workflows should be treated as an insider-threat surface.",
    updatedAt: ago(27), links: 0, sources: 1, attention: 3,
    aliases: ["CI", "counter-intel"],
  },
  {
    id: "2", name: "agents", kind: "concept", trustTier: "placeholder",
    gist: "Shorthand for AI agents — likely a near-duplicate of the AI agents concept.",
    updatedAt: ago(21), links: 1, sources: 3, attention: 1, aliases: [],
  },
  {
    id: "3", name: "Ukraine", kind: "location", trustTier: "believed",
    gist: "Frequent subject across OSINT and disinformation coverage.",
    updatedAt: ago(7), links: 0, sources: 2, attention: 1, aliases: ["UA"],
  },
  {
    id: "4", name: "Science", kind: "concept", trustTier: "believed",
    gist: "Broad topic tag; too generic to be useful — a candidate to split or drop.",
    updatedAt: ago(42), links: 0, sources: 1, attention: 1, aliases: [],
  },
  {
    id: "5", name: "intelligence community", kind: "org", trustTier: "believed",
    gist: "The U.S. intelligence community, referenced around adoption of AI inside classified workflows.",
    updatedAt: ago(27), links: 3, sources: 4, attention: 2,
    aliases: ["IC", "US IC", "the community"],
  },
  {
    id: "6", name: "AI agents", kind: "concept", trustTier: "believed",
    gist: "Software that plans and acts across tools toward a goal — the year's dominant theme and your most-sourced entity.",
    updatedAt: ago(21), links: 2, sources: 10, attention: 1,
    aliases: ["agents", "agentic AI", "LLM agents"],
  },
  {
    id: "7", name: "Signal", kind: "org", trustTier: "believed",
    gist: "Encrypted messenger — mentioned in secure-comms and OSINT contexts.",
    updatedAt: ago(35), links: 1, sources: 1, attention: 1, aliases: ["Signal app"],
  },
  {
    id: "8", name: "European Commission", kind: "org", trustTier: "believed",
    gist: "EU executive — referenced on AI regulation and GDPR enforcement.",
    updatedAt: ago(7), links: 2, sources: 5, attention: 0,
    aliases: ["EC", "the Commission"],
  },
  {
    id: "9", name: "Jensen Huang", kind: "person", trustTier: "believed",
    gist: "NVIDIA CEO — anchors the semis-supply thesis.",
    updatedAt: ago(3), links: 4, sources: 12, attention: 0, aliases: ["Jensen"],
  },
  {
    id: "10", name: "generated-code.md", kind: "other", trustTier: "believed",
    gist: "An uploaded working file collecting notes and links on agent frameworks.",
    updatedAt: ago(4), links: 0, sources: 1, attention: 0, aliases: [],
  },
];

export function EntitiesIndexSpike() {
  const theme = useAppStore((s) => s.theme);
  return (
    <div className="flex h-screen flex-col bg-bg text-ink">
      <div className="flex items-center gap-3 border-b border-line bg-chrome px-4 py-2 text-small text-ink-3">
        <span className="font-mono">/spike/entities-index</span>
        <span className="text-ink-4">·</span>
        <span>theme: {theme}</span>
        <button
          className="rounded-chip border border-line px-2 py-0.5 hover:bg-raise hover:text-ink"
          onClick={() => useAppStore.getState().setTheme(theme === "dark" ? "soft" : "dark")}
        >
          toggle theme
        </button>
      </div>
      <div className="min-h-0 flex-1 overflow-hidden">
        {/* Mirror the real chrome so the whole surface is visible, not just cards. */}
        <div className="flex h-full w-full flex-col overflow-hidden">
          <div className="border-b border-line px-5 pt-4 pb-3">
            <div className="flex items-center gap-3">
              <Icon as={Globe} className="text-ink-3" />
              <Eyebrow as="span">Entities</Eyebrow>
              <span className="font-mono text-meta text-ink-4">{MOCK.length} shown</span>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-1.5">
              <span className="rounded-chip bg-[rgb(var(--sel))] px-2.5 py-1 text-small text-ink">
                All <span className="font-mono text-meta text-ink-4">140</span>
              </span>
              <span className="rounded-chip px-2.5 py-1 text-small text-ink-3">
                Pending <span className="font-mono text-meta text-amber">9</span>
              </span>
              <span className="rounded-chip px-2.5 py-1 text-small text-ink-3">
                Unresolved <span className="font-mono text-meta text-ink-4">23</span>
              </span>
            </div>
          </div>
          <div className="flex-1 overflow-y-auto px-5 py-4">
            <div className="[column-gap:14px] columns-[250px]">
              {MOCK.map((item) => (
                <EntityCard key={item.id} item={item} onOpen={() => undefined} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
