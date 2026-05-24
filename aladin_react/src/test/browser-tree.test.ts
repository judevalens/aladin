import { describe, expect, it } from "vitest";
import type { BrowserTreeNode } from "@/shared/api/models";
import {
  ancestorFolderIds,
  buildBrowserRows,
  nextArtifactTitle,
  nextFolderTitle,
} from "@/modules/workspace/domain";

const tree: BrowserTreeNode[] = [
  {
    id: "folder-root",
    kind: "folder",
    title: "Root",
    children: [
      {
        id: "artifact-1",
        kind: "artifact",
        title: "Existing",
        artifactId: "artifact-1",
        artifactPreview: {
          id: "artifact-1",
          title: "Existing",
          kind: "note",
          updatedLabel: "today",
        },
        children: [],
      },
      {
        id: "folder-nested",
        kind: "folder",
        title: "Nested",
        children: [
          {
            id: "artifact-2",
            kind: "artifact",
            title: "Deep Existing",
            artifactId: "artifact-2",
            artifactPreview: {
              id: "artifact-2",
              title: "Deep Existing",
              kind: "note",
              updatedLabel: "today",
            },
            children: [],
          },
        ],
      },
    ],
  },
];

describe("browser tree helpers", () => {
  it("builds visible rows from expanded folders", () => {
    const rows = buildBrowserRows(tree, ["folder-root", "folder-nested"]);
    expect(rows.map((row) => row.title)).toEqual(["Root", "Existing", "Nested", "Deep Existing"]);
    expect(rows.map((row) => row.ancestorFolderIds)).toEqual([
      ["folder-root"],
      ["folder-root"],
      ["folder-root", "folder-nested"],
      ["folder-root", "folder-nested"],
    ]);
  });

  it("builds rows relative to a drilled root folder", () => {
    const rows = buildBrowserRows(tree, ["folder-nested"], "folder-root");
    expect(rows.map((row) => row.title)).toEqual(["Existing", "Nested", "Deep Existing"]);
    expect(rows.map((row) => row.depth)).toEqual([0, 0, 1]);
  });

  it("derives ancestor folder ids for a scoped folder", () => {
    expect(ancestorFolderIds(tree, "folder-nested")).toEqual(["folder-root", "folder-nested"]);
  });

  it("increments generated titles inside a folder scope", () => {
    expect(nextFolderTitle(tree, "folder-root")).toBe("New Folder");
    expect(nextArtifactTitle(tree, "folder-root", "note")).toBe("New Note");
  });
});
