import { afterEach, describe, expect, it, vi } from "vitest";
import { createBridgeHost } from "@/modules/doc-surface/bridge/bridge-host";
import type { BridgeHost } from "@/modules/doc-surface/bridge/bridge-host";

// The bridge host filters on the SOURCE WINDOW (opaque-origin frames have
// origin "null"), so the tests drive it with synthetic MessageEvents whose
// source is a stub "iframe window" capturing postMessage calls.

type Posted = { msg: Record<string, unknown>; origin: string };

function stubWindow() {
  const posted: Posted[] = [];
  const w = {
    postMessage: (msg: Record<string, unknown>, origin: string) => {
      posted.push({ msg, origin });
    },
  } as unknown as Window;
  return { w, posted };
}

function request(host: { source: Window }, body: Record<string, unknown>) {
  window.dispatchEvent(
    new MessageEvent("message", { data: { aladin: "bridge/1", type: "request", ...body }, source: host.source as unknown as MessageEventSource }),
  );
}

describe("createBridgeHost", () => {
  let host: BridgeHost | null = null;
  afterEach(() => {
    host?.detach();
    host = null;
  });

  function attach(getTheme = () => "dark") {
    const { w, posted } = stubWindow();
    host = createBridgeHost({ pageId: "artifact-1", getWindow: () => w, getTheme });
    host.attach();
    return { w, posted };
  }

  it("answers hello with protocol, theme, and capabilities", () => {
    const { w, posted } = attach(() => "cool");
    request({ source: w }, { id: 1, method: "hello" });
    expect(posted).toHaveLength(1);
    expect(posted[0].msg).toMatchObject({
      aladin: "bridge/1",
      type: "response",
      id: 1,
      ok: true,
      data: { protocol: "bridge/1", theme: "cool", capabilities: ["theme"] },
    });
  });

  it("answers theme.get with the CURRENT theme (read at answer time)", () => {
    let theme = "dark";
    const { w, posted } = attach(() => theme);
    theme = "light";
    request({ source: w }, { id: 2, method: "theme.get" });
    expect(posted[0].msg).toMatchObject({ id: 2, ok: true, data: { theme: "light" } });
  });

  it("rejects unknown methods with code unknown-method (fail fast, no timeout)", () => {
    const { w, posted } = attach();
    request({ source: w }, { id: 3, method: "nodes.get", params: { ids: ["x"] } });
    expect(posted[0].msg).toMatchObject({ id: 3, ok: false, code: "unknown-method" });
  });

  it("ignores messages from any other window (source check)", () => {
    const { posted } = attach();
    const stranger = stubWindow();
    request({ source: stranger.w }, { id: 4, method: "hello" });
    expect(posted).toHaveLength(0);
    expect(stranger.posted).toHaveLength(0);
  });

  it("ignores non-bridge envelopes from the right window", () => {
    const { w, posted } = attach();
    window.dispatchEvent(
      new MessageEvent("message", { data: { type: "aladin:bridge", cmd: "ping", id: 5 }, source: w as unknown as MessageEventSource }),
    );
    expect(posted).toHaveLength(0);
  });

  it("pushTheme pushes on the theme channel; detach stops answering", () => {
    const { w, posted } = attach();
    host!.pushTheme("soft");
    expect(posted[0].msg).toMatchObject({ type: "push", channel: "theme", data: { theme: "soft" } });
    host!.detach();
    request({ source: w }, { id: 6, method: "hello" });
    expect(posted).toHaveLength(1);
  });

  it("attach is idempotent (no double replies)", () => {
    const { w, posted } = attach();
    host!.attach();
    request({ source: w }, { id: 7, method: "hello" });
    expect(posted).toHaveLength(1);
    vi.restoreAllMocks();
  });
});
