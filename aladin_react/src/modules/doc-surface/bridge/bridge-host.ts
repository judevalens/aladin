// The host half of the shard bridge (protocol "bridge/1" — the kit's envelope).
//
// A shard iframe is sandboxed to an OPAQUE origin: postMessage is its only
// channel out, and this module is what answers. One host is attached per shard
// iframe (DocSurfaceUI); it filters strictly on the source WINDOW — the reliable
// identity check for opaque-origin frames, whose event.origin is "null" — and on
// the { aladin: "bridge/1" } envelope. Replies target "*" because an opaque
// origin cannot be named in targetOrigin; the source-window check is the gate
// (same accepted trade-off as the previous host, see SHARD_MODEL.md).
//
// M1 surface: hello / theme.get / theme push. kv.* (SHARD_LOCAL_STATE.md) and
// nodes.* (manifest-granted workspace reads) land behind the same switch.
// Unknown methods are REJECTED with code "unknown-method" so a kit call fails
// fast instead of hanging into its 8s timeout.

const BRIDGE = "bridge/1";

const METHODS = ["hello", "theme.get"] as const;
const CAPABILITIES = ["theme"] as const;

type BridgeRequest = {
  aladin?: string;
  type?: string;
  id?: number;
  method?: string;
  params?: Record<string, unknown>;
};

export interface BridgeHostDeps {
  pageId: string;
  /** The shard iframe's contentWindow (null while unmounted/reloading). */
  getWindow: () => Window | null | undefined;
  /** Current Aladin theme name; read at answer time, never cached. */
  getTheme: () => string;
}

export interface BridgeHost {
  attach(): void;
  detach(): void;
  /** Push a theme switch into the shard (kit re-stamps data-theme). */
  pushTheme(theme: string): void;
}

export function createBridgeHost(deps: BridgeHostDeps): BridgeHost {
  let listener: ((e: MessageEvent) => void) | null = null;

  function reply(to: Window, id: number | undefined, ok: boolean, body: Record<string, unknown>) {
    to.postMessage({ aladin: BRIDGE, type: "response", id, ok, ...body }, "*");
  }

  function push(channel: string, data: unknown) {
    const w = deps.getWindow();
    if (w) w.postMessage({ aladin: BRIDGE, type: "push", channel, data }, "*");
  }

  function onMessage(event: MessageEvent) {
    // Opaque-origin frames report origin "null"; the source window is the check.
    if (!event.source || event.source !== deps.getWindow()) return;
    const m = event.data as BridgeRequest | null;
    if (!m || m.aladin !== BRIDGE || m.type !== "request") return;
    const to = event.source as Window;
    switch (m.method) {
      case "hello":
        reply(to, m.id, true, {
          data: {
            protocol: BRIDGE,
            theme: deps.getTheme(),
            capabilities: [...CAPABILITIES],
            methods: [...METHODS],
          },
        });
        return;
      case "theme.get":
        reply(to, m.id, true, { data: { theme: deps.getTheme() } });
        return;
      default:
        reply(to, m.id, false, {
          error: `unknown method: ${String(m.method)}`,
          code: "unknown-method",
        });
    }
  }

  return {
    attach() {
      if (listener) return;
      listener = onMessage;
      window.addEventListener("message", listener);
    },
    detach() {
      if (!listener) return;
      window.removeEventListener("message", listener);
      listener = null;
    },
    pushTheme(theme: string) {
      push("theme", { theme });
    },
  };
}
