import { describe, expect, it } from "vitest";

import { groupEdgesByRel } from "@/modules/entities/entity-context-vocab";

describe("groupEdgesByRel", () => {
  it("groups by relation and orders sections by the REL vocabulary, not the data", () => {
    // Deliberately arrives in a jumbled order — the page's sections must be stable.
    const edges = [
      { rel: "competes", to: "c1" },
      { rel: "enables", to: "e1" },
      { rel: "competes", to: "c2" },
      { rel: "enables", to: "e2" },
    ];
    expect(groupEdgesByRel(edges)).toEqual([
      { rel: "enables", edges: [{ rel: "enables", to: "e1" }, { rel: "enables", to: "e2" }] },
      { rel: "competes", edges: [{ rel: "competes", to: "c1" }, { rel: "competes", to: "c2" }] },
    ]);
  });

  it("keeps unknown relations, trailing after the known ones", () => {
    const edges = [
      { rel: "invented_by", to: "x" },
      { rel: "enables", to: "e1" },
    ];
    expect(groupEdgesByRel(edges)).toEqual([
      { rel: "enables", edges: [{ rel: "enables", to: "e1" }] },
      { rel: "invented_by", edges: [{ rel: "invented_by", to: "x" }] },
    ]);
  });

  it("is empty for no edges", () => {
    expect(groupEdgesByRel([])).toEqual([]);
  });
});
