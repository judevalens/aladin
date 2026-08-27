import { describe, expect, it } from "vitest";
import type { BrowserTreeNode } from "@/shared/api/models";
import {
  ancestorFolderIds,
  buildBrowserRows,
  flattenFolderOptions,
  nextArtifactTitle,
  nextFolderTitle,
  rowRangeKeys,
  selectionDeleteTargets,
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

// RESEARCH_SURFACE_PRD §5: a research folder has the anatomy of a folder — it is a
// CONTAINER. Before the research kind existed, every traversal compared `kind ===
// "folder"`, so a research node rendered as a leaf and held nothing. These pin the
// container behaviour so that regression can't come back quietly.
describe("research folders are containers", () => {
  const researchTree: BrowserTreeNode[] = [
    {
      id: "folder-root",
      kind: "folder",
      title: "Ideas",
      children: [
        {
          id: "research-pead",
          kind: "research",
          title: "PEAD semis",
          research: { runState: "idle", execMode: "event", sourceKind: "authored" },
          children: [
            {
              id: "artifact-paper",
              kind: "artifact",
              title: "The paper",
              artifactId: "artifact-paper",
              artifactPreview: {
                id: "artifact-paper",
                title: "The paper",
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

  it("renders a research node as an expandable row, not a leaf", () => {
    const collapsed = buildBrowserRows(researchTree, ["folder-root"]);
    const row = collapsed.find((r) => r.id === "research-pead");
    expect(row).toBeDefined();
    expect(row?.kind).toBe("research");
    // folderId is the handle expand / drill / create-here all key off.
    expect(row?.folderId).toBe("research-pead");
    expect(row?.childCount).toBe(1);
    // Collapsed: the child must not be in the rows.
    expect(collapsed.some((r) => r.id === "artifact-paper")).toBe(false);
  });

  it("expands to reveal its children", () => {
    const expanded = buildBrowserRows(researchTree, ["folder-root", "research-pead"]);
    const child = expanded.find((r) => r.id === "artifact-paper");
    expect(child).toBeDefined();
    expect(child?.depth).toBe(2);
    expect(child?.ancestorFolderIds).toContain("research-pead");
  });

  // §11: the structural slots are typed children in the normal tree — expanding a
  // research folder gives you the notebook sidebar for free.
  it("expands to reveal its structural slots, above the captured material", () => {
    const expanded = buildBrowserRows(researchTree, ["folder-root", "research-pead"]);
    // A container's own row lists itself in ancestorFolderIds, so drop it.
    const inside = expanded.filter(
      (r) => r.id !== "research-pead" && r.ancestorFolderIds.includes("research-pead"),
    );

    expect(inside.map((r) => r.title)).toEqual([
      "Overview",
      "Manifest",
      "Runs",
      "Code",
      "The paper",
    ]);
    // Slots are derived rows carrying the view they open, not database rows.
    const overview = inside[0];
    expect(overview.kind).toBe("research-view");
    expect(overview.researchView).toBe("overview");
    expect(overview.contextId).toBe("research-pead");
    expect(overview.depth).toBe(2);
  });

  it("shows no slots while collapsed", () => {
    const collapsed = buildBrowserRows(researchTree, ["folder-root"]);
    expect(collapsed.some((r) => r.kind === "research-view")).toBe(false);
  });

  it("gives plain folders no slots", () => {
    const rows = buildBrowserRows(
      [{ id: "f1", kind: "folder", title: "Plain", children: [] }],
      ["f1"],
    );
    expect(rows.some((r) => r.kind === "research-view")).toBe(false);
  });

  it("carries the light run state so the row can show it without a fetch", () => {
    const rows = buildBrowserRows(researchTree, ["folder-root"]);
    expect(rows.find((r) => r.id === "research-pead")?.research?.runState).toBe("idle");
  });

  it("is a valid move destination, since research artifacts live inside it", () => {
    expect(flattenFolderOptions(researchTree).map((o) => o.id)).toContain("research-pead");
  });

  it("resolves as an ancestor when scoping to a descendant", () => {
    expect(ancestorFolderIds(researchTree, "research-pead")).toEqual(["folder-root", "research-pead"]);
  });
});

// ——— multi-select helpers ———
// The selection is an overlay of ROW ids; ranges follow rendered order (what the eye sees),
// and a delete batch is coalesced so the count the user confirms matches what the server's
// recursive delete actually does.

describe("row selection helpers", () => {
  const expanded = ["folder-root", "folder-nested"];
  const rows = () => buildBrowserRows(tree, expanded);
  const researchTree: BrowserTreeNode[] = [
    {
      id: "folder-root",
      kind: "folder",
      title: "Ideas",
      children: [
        {
          id: "research-pead",
          kind: "research",
          title: "PEAD semis",
          research: { runState: "idle", execMode: "event", sourceKind: "authored" },
          children: [],
        },
      ],
    },
  ];

  it("ranges over the visible rows between anchor and target, either direction", () => {
    expect(rowRangeKeys(rows(), "artifact-1", "artifact-2")).toEqual([
      "artifact-1",
      "folder-nested",
      "artifact-2",
    ]);
    expect(rowRangeKeys(rows(), "artifact-2", "artifact-1")).toEqual([
      "artifact-1",
      "folder-nested",
      "artifact-2",
    ]);
  });

  it("degrades to just the target when the anchor is gone", () => {
    expect(rowRangeKeys(rows(), "collapsed-away", "artifact-1")).toEqual(["artifact-1"]);
    expect(rowRangeKeys(rows(), null, "artifact-1")).toEqual(["artifact-1"]);
  });

  it("excludes research slot rows from a range — they are derived, not deletable", () => {
    const slotRows = buildBrowserRows(researchTree, ["folder-root", "research-pead"]);
    const keys = rowRangeKeys(slotRows, slotRows[0].id, slotRows[slotRows.length - 1].id);
    expect(keys.some((key) => key.startsWith("research:"))).toBe(false);
  });

  it("coalesces a selected child into its selected ancestor folder", () => {
    const targets = selectionDeleteTargets(rows(), ["folder-root", "artifact-2", "folder-nested"]);
    expect(targets).toEqual([
      { kind: "folder", id: "folder-root", title: "Root", childCount: 2 },
    ]);
  });

  it("keeps independent targets, artifacts resolving to their artifact id", () => {
    const targets = selectionDeleteTargets(rows(), ["folder-nested", "artifact-1"]);
    expect(targets).toEqual([
      { kind: "artifact", id: "artifact-1", title: "Existing" },
      { kind: "folder", id: "folder-nested", title: "Nested", childCount: 1 },
    ]);
  });

  it("drops keys that are not in the rendered rows", () => {
    expect(selectionDeleteTargets(rows(), ["ghost"])).toEqual([]);
  });

  it("types a research folder as research", () => {
    const researchRows = buildBrowserRows(researchTree, ["folder-root"]);
    expect(selectionDeleteTargets(researchRows, ["research-pead"])).toEqual([
      { kind: "research", id: "research-pead", title: "PEAD semis", childCount: 0 },
    ]);
  });
});
