import { describe, expect, it } from "vitest";
import {
  isAcknowledgedConflict,
  nextPageRevision,
} from "@/services/pages/page-session-service";

const draftBlocks = [
  { id: "a", type: "paragraph", content: [{ type: "text", text: "draft body" }] },
];
const serverBlocks = [
  { id: "a", type: "paragraph", content: [{ type: "text", text: "server copy" }] },
];

describe("page services", () => {
  it("increments revisions for the next save command", () => {
    expect(nextPageRevision(0)).toBe(1);
    expect(nextPageRevision(8)).toBe(9);
  });

  it("recognizes when the server already acknowledged the pending draft", () => {
    expect(
      isAcknowledgedConflict(
        { blocks: draftBlocks, revision: 4 },
        draftBlocks,
        4,
      ),
    ).toBe(true);
  });

  it("does not acknowledge unrelated server snapshots", () => {
    expect(
      isAcknowledgedConflict(
        { blocks: serverBlocks, revision: 4 },
        draftBlocks,
        4,
      ),
    ).toBe(false);
  });
});
