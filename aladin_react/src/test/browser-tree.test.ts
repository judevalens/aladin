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
        children: [],
      },
    ],
  },
];

describe("browser tree helpers", () => {
  it("builds visible rows from expanded folders", () => {
    const rows = buildBrowserRows(tree, null, ["folder-root"]);
    expect(rows.map((row) => row.title)).toEqual(["Root", "Existing", "Nested"]);
  });

  it("derives ancestor folder ids for a scoped folder", () => {
    expect(ancestorFolderIds(tree, "folder-nested")).toEqual(["folder-root", "folder-nested"]);
  });

  it("increments generated titles inside a folder scope", () => {
    expect(nextFolderTitle(tree, "folder-root")).toBe("New Folder");
    expect(nextArtifactTitle(tree, "folder-root", "note")).toBe("New Note");
  });
});
