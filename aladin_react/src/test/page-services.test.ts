import { describe, expect, it } from "vitest";
import { isAcknowledgedConflict, nextPageRevision } from "@/services/pages/page-session-service";

describe("page services", () => {
  it("increments revisions for the next save command", () => {
    expect(nextPageRevision(0)).toBe(1);
    expect(nextPageRevision(8)).toBe(9);
  });

  it("recognizes when the server already acknowledged the pending draft", () => {
    expect(
      isAcknowledgedConflict(
        { content: "draft body", revision: 4 },
        "draft body",
        4,
      ),
    ).toBe(true);
  });

  it("does not acknowledge unrelated server snapshots", () => {
    expect(
      isAcknowledgedConflict(
        { content: "server copy", revision: 4 },
        "local draft",
        4,
      ),
    ).toBe(false);
  });
});
