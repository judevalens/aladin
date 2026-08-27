import { describe, expect, it } from "vitest";

import {
  LINK_HEIGHT_BARE,
  LINK_HEIGHT_IMAGE,
  linkDomain,
  pastedUrl,
  unfurlFailedPatch,
  unfurlPatch,
  type UnfurlResult,
} from "@/modules/board/domain/board-links";

const RESULT: UnfurlResult = {
  url: "https://ssrn.com/momentum",
  domain: "ssrn.com",
  title: "Momentum Crashes",
  description: "Momentum strategies crash in panic states.",
  siteName: "SSRN",
  imageUrl: "",
  faviconUrl: "https://ssrn.com/favicon.ico",
};

describe("pastedUrl — what counts as a link on the clipboard", () => {
  it("accepts http(s) URLs and bare domains, normalizing the scheme", () => {
    expect(pastedUrl("https://ssrn.com/momentum")).toBe("https://ssrn.com/momentum");
    expect(pastedUrl("  http://example.com  ")).toBe("http://example.com");
    expect(pastedUrl("papers.ssrn.com/sol3/x?id=1")).toBe("https://papers.ssrn.com/sol3/x?id=1");
  });

  it("refuses prose, multi-token text, and non-web schemes", () => {
    expect(pastedUrl("Momentum strategies crash in panic states.")).toBeNull();
    expect(pastedUrl("see https://example.com for details")).toBeNull();
    expect(pastedUrl("hello")).toBeNull();
    expect(pastedUrl("ftp://example.com")).toBeNull();
    expect(pastedUrl("")).toBeNull();
  });
});

describe("unfurl patches", () => {
  it("ready patch carries the metadata and keeps the bare height without an image", () => {
    const patch = unfurlPatch(RESULT);
    expect(patch.status).toBe("ready");
    expect(patch.title).toBe("Momentum Crashes");
    expect(patch.h).toBe(LINK_HEIGHT_BARE);
  });

  it("grows the object when the preview has an image", () => {
    const patch = unfurlPatch({ ...RESULT, imageUrl: "https://ssrn.com/og.png" });
    expect(patch.h).toBe(LINK_HEIGHT_IMAGE);
    expect(patch.image).toBe("https://ssrn.com/og.png");
  });

  it("failed patch keeps the domain so the bare link still reads", () => {
    const patch = unfurlFailedPatch("https://www.example.com/x");
    expect(patch.status).toBe("failed");
    expect(patch.domain).toBe("example.com");
  });
});

describe("linkDomain", () => {
  it("strips www and lowercases; falls back to the raw string for junk", () => {
    expect(linkDomain("https://WWW.SSRN.com/x")).toBe("ssrn.com");
    expect(linkDomain("not a url")).toBe("not a url");
  });
});
