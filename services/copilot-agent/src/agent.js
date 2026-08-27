import { getProviderForTurn } from "./providers/index.js";

// Stable sidecar entrypoint. Individual SDK harnesses live behind provider adapters;
// the Go API and dock keep consuming the same NDJSON contract.
export async function runTurn(body, turn, writeEvent, deps = {}) {
  return getProviderForTurn(body).runTurn(body, turn, writeEvent, deps);
}
