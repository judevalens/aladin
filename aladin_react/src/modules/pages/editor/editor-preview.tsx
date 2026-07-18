import { BlockNoteView } from "@blocknote/shadcn";
import { useCreateBlockNote } from "@blocknote/react";
import { useState } from "react";
import "@blocknote/shadcn/style.css";

import { pageSchema } from "@/modules/pages/editor/entity-mention";
import { useAppStore } from "@/app/state/store";

// Dev-only editor preview (route: /spike/editor). Mounts the real pageSchema
// WITHOUT collaboration or auth so the editor's look-and-feel can be iterated on
// and screenshotted outside the login wall. Not part of the shipped app.
//
// Seeded with the block + inline types a research log leans on, so every styled
// surface is visible at once: headings, lists, quote, code, table, divider, and
// the @entity / # ref inline chips.
const seed = [
  { type: "heading", props: { level: 1 }, content: "NVDA — earnings thesis" },
  {
    type: "paragraph",
    content: [
      { type: "text", text: "Core view: ", styles: {} },
      { type: "text", text: "data-center demand", styles: { bold: true } },
      { type: "text", text: " stays supply-constrained through H2. Tracking ", styles: {} },
      { type: "entityMention", props: { entityId: "e1", label: "NVDA", kind: "asset" } },
      { type: "text", text: " against ", styles: {} },
      { type: "artifactRef", props: { kind: "claim", targetId: "c1", label: "hyperscaler capex holds", polarity: "for" } },
      { type: "text", text: ".", styles: {} },
    ],
  },
  { type: "heading", props: { level: 2 }, content: "Evidence for" },
  { type: "bulletListItem", content: "Cloud capex guides revised up 3 quarters running" },
  { type: "bulletListItem", content: "Lead times still 8–12 weeks on top SKUs" },
  { type: "heading", props: { level: 2 }, content: "Open questions" },
  { type: "checkListItem", props: { checked: true }, content: "Confirm gross-margin trajectory" },
  { type: "checkListItem", props: { checked: false }, content: "China export-control exposure" },
  {
    type: "quote",
    content: "The risk isn't demand — it's whether they can keep pricing power once supply catches up.",
  },
  { type: "heading", props: { level: 3 }, content: "Entry plan" },
  {
    type: "codeBlock",
    props: { language: "python" },
    content: "signal = close > sma(close, 50) and rsi(14) < 70\nsize = 0.02 * equity / atr(14)",
  },
  {
    type: "table",
    content: {
      type: "tableContent",
      rows: [
        { cells: ["Scenario", "Target", "Prob"] },
        { cells: ["Bull", "+18%", "40%"] },
        { cells: ["Base", "+6%", "45%"] },
        { cells: ["Bear", "-12%", "15%"] },
      ],
    },
  },
  { type: "paragraph", content: "Numbered checklist before entry:" },
  { type: "numberedListItem", content: "Confirm trend on the daily" },
  { type: "numberedListItem", content: "Size off ATR, cap at 2% risk" },
  { type: "numberedListItem", content: "Set the stop before the order" },
  { type: "paragraph", content: "" },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
] as any;

export function EditorPreview() {
  const theme = useAppStore((s) => s.theme);
  const [width, setWidth] = useState(720);
  const editor = useCreateBlockNote({ schema: pageSchema, initialContent: seed });

  return (
    <div className="flex h-screen flex-col bg-bg text-ink">
      <div className="flex items-center gap-3 border-b border-line bg-chrome px-4 py-2 text-xs text-ink-3">
        <span className="font-mono">/spike/editor</span>
        <span className="text-ink-4">·</span>
        <span>theme: {theme}</span>
        <button
          className="rounded-chip border border-line px-2 py-0.5 hover:bg-raise hover:text-ink"
          onClick={() => useAppStore.getState().setTheme(theme === "dark" ? "soft" : "dark")}
        >
          toggle theme
        </button>
        <div className="ml-auto flex items-center gap-2">
          {[640, 720, 860].map((w) => (
            <button
              key={w}
              className={`rounded-chip border px-2 py-0.5 ${
                width === w ? "border-amber-line bg-amber-soft text-amber" : "border-line hover:bg-raise"
              }`}
              onClick={() => setWidth(w)}
            >
              {w}px
            </button>
          ))}
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto px-8 py-10" style={{ maxWidth: width }}>
          <BlockNoteView editor={editor} theme="dark" />
        </div>
      </div>
    </div>
  );
}
