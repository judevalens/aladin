import { BlockNoteEditor } from "@blocknote/core";
import { BlockNoteView } from "@blocknote/shadcn";
import { BlockNoteViewRaw, useCreateBlockNote } from "@blocknote/react";
import { render, screen } from "@testing-library/react";
import { DOMSerializer, Node as PMNode } from "prosemirror-model";
import { describe, expect, it } from "vitest";

function BlockNoteSmokeHarness() {
  const editor = useCreateBlockNote();
  return <BlockNoteView editor={editor} theme="light" />;
}

function BlockNoteRawSmokeHarness() {
  const editor = useCreateBlockNote();
  return <BlockNoteViewRaw editor={editor} theme="light" />;
}

describe("BlockNote smoke", () => {
  it("renders a shadcn editor", () => {
    render(<BlockNoteSmokeHarness />);
    expect(screen.getAllByRole("textbox").length).toBeGreaterThan(0);
  });

  it("renders a raw editor view", () => {
    render(<BlockNoteRawSmokeHarness />);
    expect(screen.getAllByRole("textbox").length).toBeGreaterThan(0);
  });

  it("has renderable DOM specs for the initial ProseMirror document", () => {
    const editor = BlockNoteEditor.create();
    const failures: string[] = [];

    editor.prosemirrorState.doc.descendants((node: PMNode) => {
      const toDOM = node.type.spec.toDOM;
      if (!toDOM) return;
      try {
        const spec = toDOM(node);
        DOMSerializer.renderSpec(document, spec);
      } catch (error) {
        failures.push(
          `${node.type.name}: ${error instanceof Error ? error.message : String(error)}`,
        );
      }
    });

    expect(failures).toEqual([]);
  });
});
