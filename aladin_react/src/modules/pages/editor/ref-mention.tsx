import { createReactInlineContentSpec } from "@blocknote/react";

import type { ArtifactRef, RefKind } from "@/modules/graph/graph-pane-types";

// Per-kind chip styling for a `#` reference — navigational artifact links. Keyed loosely
// so a legacy chip stored with a retired kind (e.g. the removed claim layer) still renders
// rather than coming out unstyled.
const refChipClass: Record<string, string> = {
  page: "bg-raise text-ink-2",
  shard: "bg-raise text-echo",
};
const refChipFallback = "bg-raise text-ink-3";

// artifactRef is an inline `#` reference to a page or shard. It carries the target kind +
// id so the page's refs can be projected into artifact_refs; label is for display. Chips
// navigate on click (via a delegated handler in the driver, keyed off the data-* attributes).
// `polarity` is retained in the prop schema only so documents written against the old claim
// layer still parse.
export const artifactRefSpec = createReactInlineContentSpec(
  {
    type: "artifactRef",
    propSchema: {
      kind: { default: "" },
      targetId: { default: "" },
      label: { default: "" },
      polarity: { default: "" },
    },
    content: "none",
  } as const,
  {
    render: ({ inlineContent }) => {
      const { kind, targetId, label } = inlineContent.props;
      const cls = refChipClass[kind] ?? refChipFallback;
      const clickable = kind === "page" || kind === "shard";
      return (
        <span
          data-ref-kind={kind}
          data-ref-target={targetId}
          role={clickable ? "button" : undefined}
          className={`rounded-chip px-1 py-0.5 font-medium ${cls} ${clickable ? "cursor-pointer hover:underline" : ""}`}
          title={kind === "claim" ? "claim" : kind}
        >
          #{label}
        </span>
      );
    },
  },
);

// extractArtifactRefs walks a BlockNote document and pulls out every `#` reference as an
// ArtifactRef, deduped by (kind+target, block). This is the projection the page sends to the
// backend so artifact_refs stays in sync with the doc.
export function extractArtifactRefs(blocks: unknown): ArtifactRef[] {
  const out: ArtifactRef[] = [];
  const seen = new Set<string>();

  const walk = (nodes: unknown) => {
    if (!Array.isArray(nodes)) return;
    for (const node of nodes) {
      if (!node || typeof node !== "object") continue;
      const block = node as { id?: string; content?: unknown; children?: unknown };
      if (Array.isArray(block.content)) {
        for (const ic of block.content) {
          if (ic && typeof ic === "object" && (ic as { type?: string }).type === "artifactRef") {
            const props =
              (ic as { props?: { kind?: string; targetId?: string; label?: string } }).props ?? {};
            const kind = props.kind ?? "";
            const targetId = props.targetId ?? "";
            if (!targetId || (kind !== "claim" && kind !== "page" && kind !== "shard")) continue;
            const blockId = block.id ?? "";
            const key = `${kind} ${targetId} ${blockId}`;
            if (seen.has(key)) continue;
            seen.add(key);
            out.push({ kind: kind as RefKind, targetId, blockId, surface: props.label ?? "" });
          }
        }
      }
      walk(block.children);
    }
  };

  walk(blocks);
  return out;
}
