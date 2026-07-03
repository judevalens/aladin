// Server-side BlockNote schema for Aladin pages. It MUST mirror the client schema in
// aladin_react/src/modules/pages/editor/entity-mention.tsx — same inline node `type`,
// `propSchema`, and `content` mode.
//
// Why this exists: the collab server round-trips the live Y.Doc through a headless
// ServerBlockNoteEditor (yXmlFragmentToBlocks / blocksToYXmlFragment, and the projection /
// history reads). With the DEFAULT schema, the custom `entityMention` node is unknown and
// gets dropped on that round-trip — so an @entity inserted on the client reappears for a
// moment, then the server reconciles it away. Teaching the server the node keeps it intact.

import { BlockNoteSchema, createInlineContentSpec, defaultInlineContentSpecs } from "@blocknote/core";

// entityMention — an inline reference to a resolved entity (vanilla spec; server has no
// React). render is only used for HTML/markdown serialization; the Y.Doc round-trip relies
// on the node being present in the schema with this propSchema.
const entityMention = createInlineContentSpec(
  {
    type: "entityMention",
    propSchema: {
      entityId: { default: "" },
      label: { default: "" },
      kind: { default: "" },
    },
    content: "none",
  },
  {
    render: (inlineContent) => {
      const dom = document.createElement("span");
      dom.className = "entity-mention";
      dom.setAttribute("data-entity-id", inlineContent.props.entityId || "");
      dom.textContent = "@" + (inlineContent.props.label || "");
      return { dom };
    },
  },
);

// artifactRef — an inline `#` reference to a claim, page, or shard. Same mirror rationale as
// entityMention: the server must know the node so the Y.Doc round-trip doesn't drop it.
const artifactRef = createInlineContentSpec(
  {
    type: "artifactRef",
    propSchema: {
      kind: { default: "" },
      targetId: { default: "" },
      label: { default: "" },
      polarity: { default: "" },
    },
    content: "none",
  },
  {
    render: (inlineContent) => {
      const dom = document.createElement("span");
      dom.className = "artifact-ref";
      dom.setAttribute("data-ref-kind", inlineContent.props.kind || "");
      dom.setAttribute("data-ref-target", inlineContent.props.targetId || "");
      dom.textContent = "#" + (inlineContent.props.label || "");
      return { dom };
    },
  },
);

export const pageSchema = BlockNoteSchema.create({
  inlineContentSpecs: {
    ...defaultInlineContentSpecs,
    entityMention,
    artifactRef,
  },
});
