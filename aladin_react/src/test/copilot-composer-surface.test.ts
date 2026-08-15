import { describe, expect, it } from "vitest";
import {
  composerPlaceholder,
  describeSurface,
  scopeForSurface,
  suggestionsFor,
  surfaceKindLabel,
} from "@/modules/copilot/ui/copilot-surface";

describe("copilot composer surface helpers", () => {
  it("uses concrete surface names in the composer placeholder", () => {
    expect(composerPlaceholder(false, "Collar payoff")).toBe("Ask about Collar payoff…");
    expect(composerPlaceholder(false, null)).toBe("Ask the copilot…");
    expect(composerPlaceholder(true, "Collar payoff")).toBe("Type a follow-up — sends when this turn finishes…");
  });

  it("describes artifact kinds with user-facing labels", () => {
    expect(describeSurface({ kind: "artifact", id: "a1", label: "Collar payoff", artifactKind: "app" })).toBe(
      "Collar payoff",
    );
    expect(describeSurface({ kind: "artifact", id: "a2", artifactKind: "file" })).toBe("this source");
    expect(describeSurface({ kind: "markets" })).toBe("Markets");
    expect(surfaceKindLabel("app")).toBe("shard");
    expect(surfaceKindLabel("note")).toBe("page");
  });

  it("tailors empty-state suggestions by surface", () => {
    expect(suggestionsFor({ kind: "ticker", symbol: "NVDA" })).toEqual([
      "What's my thesis on NVDA?",
      "How does NVDA look technically?",
      "Any recent notes on NVDA?",
    ]);
    expect(suggestionsFor({ kind: "artifact", artifactKind: "app" })).toContain("Polish the interaction design");
    expect(suggestionsFor({ kind: "artifact", artifactKind: "file" })).toContain("Summarize this source");
  });

  it("builds scope summaries for ticker, shard, and market surfaces", () => {
    expect(scopeForSurface({ kind: "ticker", symbol: "nvda" }, "nvda")).toMatchObject({
      title: "NVDA",
      kind: "ticker",
      rows: [{ label: "symbol", value: "NVDA" }],
    });
    expect(scopeForSurface({ kind: "artifact", id: "shd_1", artifactKind: "app" }, "Collar payoff")).toMatchObject({
      title: "Collar payoff",
      kind: "shard",
      rows: [
        { label: "id", value: "shd_1" },
        { label: "type", value: "app" },
      ],
    });
    expect(scopeForSurface({ kind: "markets" }, "Markets")).toMatchObject({
      title: "Markets",
      kind: "market surface",
      rows: [{ label: "scope", value: "watchlists" }],
    });
  });
});
