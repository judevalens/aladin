// Guards the reference <-> markdown codec: @entity / #ref nodes must survive serialization
// to markdown (as aladin-scheme links) and parse back into the same nodes — so agents that
// read/author pages as markdown can see and create references.

import { test } from "node:test";
import assert from "node:assert/strict";

import { blocksToMd, mdToBlocks } from "../src/services/converter.js";
import { decodeLinksToRefs, encodeRefsToLinks } from "../src/services/ref-codec.js";

test("encode/decode is a round-trip for @entity and #ref nodes", () => {
  const blocks = [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "per ", styles: {} },
        { type: "entityMention", props: { entityId: "e-1", label: "OpenAI", kind: "org" } },
        { type: "text", text: " and ", styles: {} },
        { type: "artifactRef", props: { kind: "claim", targetId: "c-9", label: "AGI by 2030", polarity: "assert" } },
      ],
    },
  ];

  const decoded = decodeLinksToRefs(encodeRefsToLinks(blocks));
  const inline = decoded[0].content;
  assert.equal(inline[1].type, "entityMention");
  assert.equal(inline[1].props.entityId, "e-1");
  assert.equal(inline[1].props.label, "OpenAI");
  assert.equal(inline[3].type, "artifactRef");
  assert.equal(inline[3].props.kind, "claim");
  assert.equal(inline[3].props.targetId, "c-9");
  assert.equal(inline[3].props.label, "AGI by 2030");
});

test("blocksToMd emits aladin-scheme links for references", async () => {
  const blocks = [
    {
      type: "paragraph",
      content: [
        { type: "entityMention", props: { entityId: "e-1", label: "OpenAI", kind: "org" } },
        { type: "text", text: " ", styles: {} },
        { type: "artifactRef", props: { kind: "page", targetId: "art_7", label: "Roadmap", polarity: "" } },
      ],
    },
  ];
  const md = await blocksToMd(blocks);
  assert.match(md, /\[@OpenAI\]\(https:\/\/aladin\.ref\/entity\/e-1\)/);
  assert.match(md, /\[#Roadmap\]\(https:\/\/aladin\.ref\/page\/art_7\)/);
});

test("mdToBlocks parses aladin.ref links into reference nodes", async () => {
  const md =
    "see [#AGI by 2030](https://aladin.ref/claim/c-9) per [@OpenAI](https://aladin.ref/entity/e-1)";
  const blocks = await mdToBlocks(md);
  const inline = blocks.flatMap((b) => (Array.isArray(b.content) ? b.content : []));
  const claim = inline.find((ic) => ic.type === "artifactRef");
  const entity = inline.find((ic) => ic.type === "entityMention");
  assert.ok(claim, "aladin:claim link should become an artifactRef");
  assert.equal(claim.props.kind, "claim");
  assert.equal(claim.props.targetId, "c-9");
  assert.equal(claim.props.label, "AGI by 2030");
  assert.ok(entity, "aladin:entity link should become an entityMention");
  assert.equal(entity.props.entityId, "e-1");
  assert.equal(entity.props.label, "OpenAI");
});

test("a literal '#' heading or '@' in prose is left untouched (no false refs)", async () => {
  const md = "# Real Heading\n\nemail me @ noon about C# and #1 pick";
  const blocks = await mdToBlocks(md);
  const inline = blocks.flatMap((b) => (Array.isArray(b.content) ? b.content : []));
  assert.equal(
    inline.find((ic) => ic.type === "entityMention" || ic.type === "artifactRef"),
    undefined,
    "literal @/# must not be parsed as references",
  );
  assert.ok(
    blocks.some((b) => b.type === "heading"),
    "a markdown heading should still parse as a heading",
  );
});
