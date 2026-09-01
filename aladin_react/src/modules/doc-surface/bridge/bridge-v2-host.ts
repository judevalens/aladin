import { validateWithSchema } from "../data/schema-validation";
import { decodeJSON } from "../data/schema-profile";
import requestSchema from "../../../../../shared/shard-v2/schemas/bridge-request.json";
import type { BridgeHost } from "./bridge-host";
import type { HostResourceSession, HostResourceTarget, ResourceHostHub } from "./resource-host-hub";

export function createBridgeV2Host(deps: { target: HostResourceTarget; buildId: string; getWindow: () => Window | null | undefined; getTheme: () => string; hub: ResourceHostHub }): BridgeHost {
  let session: HostResourceSession | undefined;
  let generation = 0;
  let inflight = 0;
  const push = (channel: string, data: unknown) => deps.getWindow()?.postMessage({ aladin: "bridge/2", type: "push", channel, data }, "*");
  const onMessage = (event: MessageEvent) => {
    if (!session || event.source !== deps.getWindow() || !event.source) return;
    const raw = event.data as Record<string, unknown> | null;
    if (raw?.aladin !== "bridge/2" || raw.type !== "request") return;
    const to = event.source as Window;
    const current = generation;
    const respond = (ok: boolean, data: unknown) => {
      if (current !== generation || !session || to !== deps.getWindow()) return;
      to.postMessage({ aladin: "bridge/2", type: "response", id: raw.id, ok, ...(ok ? { data } : data as object) }, "*");
    };
    const fail = (error: unknown) => {
      const e = error as { code?: string; message?: string };
      respond(false, { code: e.code ?? "bad-request", error: e.message ?? "Invalid resource request" });
    };
    try {
      const request = decodeJSON(JSON.stringify(raw)) as { method: string; params: Record<string, unknown> };
      validateWithSchema(requestSchema, request);
      if (inflight >= 32) throw { code: "rate-limited", message: "Too many pending requests" };
      if (request.method === "theme.get") { respond(true, { theme: deps.getTheme() }); return; }
      inflight++;
      void session.call(request.method, request.params).then(data => {
        if (request.method === "hello") {
          const hello = data as { buildId?: string; contractHash?: string };
          if (hello.buildId !== deps.buildId || hello.contractHash !== deps.target.contractHash) throw { code: "contract-changed", message: "Reload the shard's release" };
          respond(true, { ...hello, theme: deps.getTheme() });
        } else respond(true, data);
      }).catch(fail).finally(() => { inflight--; });
    } catch (error) { fail(error); }
  };
  return {
    attach() { if (session) return; session = deps.hub.session(deps.target, push); window.addEventListener("message", onMessage); },
    detach() { generation++; window.removeEventListener("message", onMessage); session?.close(); session = undefined; },
    pushTheme(theme) { if (session) push("theme", { theme }); },
  };
}
