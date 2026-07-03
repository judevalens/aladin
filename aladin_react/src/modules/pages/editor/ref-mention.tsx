import { createReactInlineContentSpec } from "@blocknote/react";

import type { ArtifactRef, RefKind } from "@/modules/graph/graph-pane-types";

// Per-kind chip styling for a `#` reference. Claims are the engine-reasoned thesis layer
// (accent-tinted); pages/shards are navigational artifact links.
const refChipClass: Record<RefKind, string> = {
  claim: "bg-raise text-catalyst",
  page: "bg-raise text-ink-2",
  shard: "bg-raise text-echo",
};

// artifactRef is an inline `#` reference to a claim, page, or shard. It carries the target
// kind + id so the page's refs can be projected into artifact_refs; label is for display,
// polarity annotates a claim chip. Pages/shards navigate on click (via delegated handler in
// the driver, keyed off the data-* attributes); claim chips are display-only for now.
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
      const cls = refChipClass[(kind as RefKind)] ?? "bg-raise text-ink-2";
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
