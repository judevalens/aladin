import { ResourceClient } from "./resource-client";
import { BridgeResourceTransport, WindowResourceBridgePort } from "./bridge-transport";
import { createUseResource } from "./use-resource";
import type { Binding, Data, Query, ResourceSnapshot } from "./types";
export { allocateBridgeRequestID } from "./bridge-transport";
export { resourceRequestId } from "./request-id";

let hook: ReturnType<typeof createUseResource> | undefined;
let transport: BridgeResourceTransport | undefined;

/** One lazy client per iframe; v1 kit imports never compile a v2 schema. */
function ensureSession() {
  if (!hook) {
    const raw = document.getElementById("aladin-resource-bootstrap")?.textContent;
    if (!raw) throw new Error("useResource requires a built Shard v2 contract");
    const boot = JSON.parse(raw) as { protocol: string; bindings: Record<string, Binding>; contractHash: string; buildId: string };
    if (boot.protocol !== "bridge/2") throw new Error("Unsupported shard protocol");
    const port = new WindowResourceBridgePort(window, window.parent);
    transport = new BridgeResourceTransport(port, { contractHash: boot.contractHash, buildId: boot.buildId });
    const client = new ResourceClient(transport, boot.bindings);
    hook = createUseResource(client);
    window.addEventListener("pagehide", () => { client.close(); port.close(); hook = undefined; transport = undefined; }, { once: true });
  }
}

export function useResource<T extends Data = Data>(binding: string, inputs: Data = {}) {
  ensureSession();
  return hook!<T>(binding, inputs);
}

/** Bounded server query/pagination, separate from the binding's live view. */
export async function queryResource<T extends Data = Data>(binding: string, query: Query, inputs: Data = {}, signal?: AbortSignal): Promise<ResourceSnapshot<T>> {
  ensureSession();
  return transport!.query<T>({ binding, inputs }, query, signal);
}

/** Execute a release-pinned persisted GraphQL operation. Raw query text is not accepted. */
export async function executeGraphQL<T = unknown>(operationId: string, variables: Data = {}, signal?: AbortSignal): Promise<{ data?: T; errors?: { message: string }[] }> {
  ensureSession();
  return transport!.executeGraphQL<T>(operationId, variables, signal);
}

/** Invoke a manual lambda from the same immutable runtime release. */
export async function invokeLambda<T = unknown>(name: string, input: Data = {}, signal?: AbortSignal): Promise<T> {
  ensureSession();
  return transport!.invokeLambda<T>(name, input, signal);
}
