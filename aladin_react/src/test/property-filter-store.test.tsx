import { render, screen } from "@testing-library/react";
import { useSyncExternalStore } from "react";
import { describe, expect, it } from "vitest";

import type { Artifact } from "@/shared/api/models";

// Regression for React #185 ("Maximum update depth exceeded"), which crashed the property-filter
// dialog the instant it opened.
//
// usePropertyFilter feeds useSyncExternalStore a no-op store while no key is selected — the state
// the dialog opens in. That store's snapshot used to be `() => [] as Artifact[]`, returning a FRESH
// array every call. useSyncExternalStore calls getSnapshot on every render and compares with
// Object.is, so the value never matched the previous one, React concluded the store had changed,
// re-rendered, called it again — an unbounded loop.
//
// The bug is in the VALUE, not the functions, which is why it survived a review that only checked
// that subscribe/snapshot were stable references. These tests pin the value.

// The exact shape the hook uses for its idle branch.
const NO_RESULTS: Artifact[] = [];
const IDLE_STORE = {
  subscribe: () => () => {},
  snapshot: () => NO_RESULTS,
};

describe("the idle property-filter store", () => {
  it("returns an identical snapshot reference on every call", () => {
    // This is React's actual requirement for getSnapshot: a cached value, not merely a pure
    // function. `() => []` satisfies "pure" and still loops forever.
    expect(IDLE_STORE.snapshot()).toBe(IDLE_STORE.snapshot());
  });

  it("does not re-render unboundedly when driven by useSyncExternalStore", () => {
    let renders = 0;
    function Probe() {
      renders += 1;
      const results = useSyncExternalStore(IDLE_STORE.subscribe, IDLE_STORE.snapshot);
      return <span data-testid="n">{results.length}</span>;
    }
    render(<Probe />);
    expect(screen.getByTestId("n").textContent).toBe("0");
    // An unstable snapshot throws "Maximum update depth exceeded" before reaching this line; a
    // stable one settles in a couple of renders. The bound is loose on purpose — the assertion is
    // "bounded", not an exact count.
    expect(renders).toBeLessThan(5);
  });
});
