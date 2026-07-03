// Guards the @entity-mention fix: the server-side editor must round-trip a custom
// `entityMention` inline node through the Y.Doc fragment without stripping it. With the
// default schema this node is unknown and gets dropped — which made @mentions vanish on
// the client after the collab server reconciled the doc.

import { test } from "node:test";
import assert from "node:assert/strict";
import * as Y from "yjs";
import { ServerBlockNoteEditor } from "@blocknote/server-util";

import { pageSchema } from "../src/services/page-schema.js";

const FRAGMENT = "document";

test("entityMention survives the server Y.Doc round-trip with pageSchema", () => {
  const editor = ServerBlockNoteEditor.create({ schema: pageSchema });

  const blocks = [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "hello ", styles: {} },
        { type: "entityMention", props: { entityId: "e-openai", label: "OpenAI", kind: "org" } },
      ],
    },
  ];

  const ydoc = new Y.Doc();
  const fragment = ydoc.getXmlFragment(FRAGMENT);
  editor.blocksToYXmlFragment(blocks, fragment);

  const out = editor.yXmlFragmentToBlocks(fragment);

  const inline = out.flatMap((b) => (Array.isArray(b.content) ? b.content : []));
  const mention = inline.find((ic) => ic.type === "entityMention");
  assert.ok(mention, "entityMention should survive the round-trip");
  assert.equal(mention.props.entityId, "e-openai");
  assert.equal(mention.props.label, "OpenAI");
});

test("artifactRef survives the server Y.Doc round-trip with pageSchema", () => {
  const editor = ServerBlockNoteEditor.create({ schema: pageSchema });

  const blocks = [
    {
      type: "paragraph",
      content: [
        { type: "text", text: "see ", styles: {} },
        {
          type: "artifactRef",
          props: { kind: "claim", targetId: "c-123", label: "AGI by 2030", polarity: "assert" },
        },
      ],
    },
  ];

  const ydoc = new Y.Doc();
  const fragment = ydoc.getXmlFragment(FRAGMENT);
  editor.blocksToYXmlFragment(blocks, fragment);

  const out = editor.yXmlFragmentToBlocks(fragment);

  const inline = out.flatMap((b) => (Array.isArray(b.content) ? b.content : []));
  const ref = inline.find((ic) => ic.type === "artifactRef");
  assert.ok(ref, "artifactRef should survive the round-trip");
  assert.equal(ref.props.kind, "claim");
  assert.equal(ref.props.targetId, "c-123");
  assert.equal(ref.props.label, "AGI by 2030");
});
