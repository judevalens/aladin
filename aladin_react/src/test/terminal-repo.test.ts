import { beforeEach, describe, expect, it, vi } from "vitest";

const invokeMock = vi.fn().mockResolvedValue(undefined);
const channelHandlers: Array<(event: unknown) => void> = [];

vi.mock("@tauri-apps/api/core", () => ({
  invoke: (...args: unknown[]) => invokeMock(...args),
  // Mirror the real Channel: constructor takes the onmessage handler.
  Channel: class {
    onmessage: ((event: unknown) => void) | null = null;
    constructor(handler?: (event: unknown) => void) {
      if (handler) {
        this.onmessage = handler;
        channelHandlers.push(handler);
      }
    }
  },
}));

import { createTerminalRepo } from "@/repos/terminal/terminal-repo";

describe("terminal-repo", () => {
  beforeEach(() => {
    invokeMock.mockClear();
    channelHandlers.length = 0;
  });

  it("opens a session with id, output+control channels, and options; routes bytes and exit", async () => {
    const repo = createTerminalRepo();
    const chunks: Uint8Array[] = [];
    const exits: Array<number | null> = [];
    await repo.open(
      "sess-1",
      { cols: 80, rows: 24 },
      (bytes) => chunks.push(bytes),
      (code) => exits.push(code),
    );

    expect(invokeMock).toHaveBeenCalledTimes(1);
    const [command, args] = invokeMock.mock.calls[0] as [string, Record<string, unknown>];
    expect(command).toBe("terminal_open");
    expect(args.id).toBe("sess-1");
    expect(args.input).toEqual({ cols: 80, rows: 24 });
    expect(args.output).toBeDefined();
    expect(args.control).toBeDefined();

    // channelHandlers[0] is the output channel, [1] is control.
    channelHandlers[0](new Uint8Array([104, 105]).buffer);
    expect(chunks).toHaveLength(1);
    expect(Array.from(chunks[0])).toEqual([104, 105]);

    channelHandlers[1]({ type: "exit", payload: { code: 0 } });
    expect(exits).toEqual([0]);
  });

  it("maps write/resize/close to their commands", async () => {
    const repo = createTerminalRepo();
    await repo.write("sess-1", "ls\n");
    await repo.resize("sess-1", 120, 40);
    await repo.close("sess-1");

    expect(invokeMock.mock.calls).toEqual([
      ["terminal_write", { id: "sess-1", data: "ls\n" }],
      ["terminal_resize", { id: "sess-1", cols: 120, rows: 40 }],
      ["terminal_close", { id: "sess-1" }],
    ]);
  });
});
