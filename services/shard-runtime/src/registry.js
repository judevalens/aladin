import { Worker } from "node:worker_threads";
import { randomUUID } from "node:crypto";
import { buildSchema, parse, TypeInfo, visit, visitWithTypeInfo } from "graphql";

const workerURL = new URL("./worker.js", import.meta.url);

export function releaseMemoryMiB(release) {
  const handlers = [
    ...Object.values(release.manifest?.graphql?.resolvers || {}),
    ...Object.values(release.manifest?.lambdas || {})
  ];
  const budgets = handlers.map((value) => Number(value?.budget?.memoryMiB)).filter((value) => Number.isFinite(value) && value > 0);
  return budgets.length ? Math.max(16, Math.min(256, ...budgets)) : 256;
}

export function releaseOperationTimeouts(release) {
  const output = new Map();
  if (!release.manifest?.graphql) return output;
  const schema = buildSchema(release.schema);
  const resolvers = release.manifest.graphql.resolvers || {};
  for (const [operationId, operation] of Object.entries(release.manifest.graphql.operations || {})) {
    const budgets = [];
    const typeInfo = new TypeInfo(schema);
    visit(parse(operation.document), visitWithTypeInfo(typeInfo, { Field(node) {
      const parent = typeInfo.getParentType();
      const qualified = parent ? `${parent.name}.${node.name.value}` : "";
      const definition = resolvers[qualified] || resolvers[node.name.value];
      if (definition?.budget?.timeoutMs) budgets.push(definition.budget.timeoutMs);
    } }));
    output.set(operationId, budgets.length ? Math.min(...budgets) : 1000);
  }
  return output;
}

class ReleaseWorker {
  constructor(release, capabilityExecutor) {
    this.release = release;
    this.capabilityExecutor = capabilityExecutor;
    this.pending = new Map();
    this.inflight = 0;
    this.draining = false;
    this.closed = false;
    this.requests = 0;
    this.operationTimeouts = releaseOperationTimeouts(release);
    this.worker = new Worker(workerURL, { workerData: release, env: {}, resourceLimits: { maxOldGenerationSizeMb: releaseMemoryMiB(release) } });
    this.worker.on("message", (message) => this.#message(message));
    this.worker.on("error", (error) => this.#fail(error));
    this.worker.on("exit", (code) => this.#fail(new Error(`resolver worker exited (${code})`)));
  }

  ready(timeoutMs = 5000) {
    return this.#request("ready", {}, timeoutMs);
  }

  execute(payload, timeoutMs) {
    this.inflight++;
    this.requests++;
    return this.#request("execute", payload, timeoutMs).finally(() => {
      this.inflight--;
      if (this.requests >= (this.release.maxRequests || 1000)) this.draining = true;
      if (this.draining && this.inflight === 0) this.close();
    });
  }

  timeoutFor(operationId) {
    return this.operationTimeouts.get(operationId) || 1000;
  }

  invokeLambda(payload, timeoutMs) {
    this.inflight++;
    this.requests++;
    return this.#request("lambda", payload, timeoutMs).finally(() => {
      this.inflight--;
      if (this.requests >= (this.release.maxRequests || 1000)) this.draining = true;
      if (this.draining && this.inflight === 0) this.close();
    });
  }

  drain() {
    this.draining = true;
    if (this.inflight === 0) this.close();
  }

  close(error = new Error("resolver worker is closed")) {
    if (this.closed) return;
    this.closed = true;
    for (const { reject, timer } of this.pending.values()) {
      clearTimeout(timer);
      reject(error);
    }
    this.pending.clear();
    this.worker.terminate();
  }

  #request(type, payload, timeoutMs) {
    if (this.closed) return Promise.reject(new Error("resolver worker is closed"));
    const id = randomUUID();
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        this.close();
        reject(Object.assign(new Error("resolver execution timed out"), { code: "TIMEOUT" }));
      }, timeoutMs);
      this.pending.set(id, { resolve, reject, timer });
      this.worker.postMessage({ type, id, ...payload });
    });
  }

  async #message(message) {
    if (message.type === "handler-timeout") {
      this.close(Object.assign(new Error("resolver execution timed out"), { code: "TIMEOUT" }));
      return;
    }
    if (message.type === "capability") {
      try {
        const result = await this.capabilityExecutor({
          releaseHash: this.release.releaseHash,
          handler: message.handler,
          capability: message.capability,
          input: message.input,
          scopeToken: message.scopeToken
        });
        if (!this.closed) this.worker.postMessage({ type: "capability-result", id: message.id, ok: true, result });
      } catch (error) {
        if (!this.closed) this.worker.postMessage({ type: "capability-result", id: message.id, ok: false, code: error?.code, error: String(error?.message || error) });
      }
      return;
    }
    const pending = this.pending.get(message.id);
    if (!pending) return;
    clearTimeout(pending.timer);
    this.pending.delete(message.id);
    if (message.ok) pending.resolve(message.result);
    else pending.reject(Object.assign(new Error(message.error || "resolver execution failed"), { code: message.code || "EXECUTION_FAILED" }));
  }

  #fail(error) {
    if (this.closed) return;
    this.closed = true;
    for (const { reject, timer } of this.pending.values()) {
      clearTimeout(timer);
      reject(error);
    }
    this.pending.clear();
  }
}

export class ReleaseRegistry {
  constructor({ capabilityExecutor = async () => { throw new Error("capability executor is not configured"); }, prepareTimeoutMs = 5000 } = {}) {
    this.capabilityExecutor = capabilityExecutor;
    this.prepareTimeoutMs = prepareTimeoutMs;
    this.candidates = new Map();
    this.active = new Map();
  }

  async prepare(release) {
    if (!release?.scopeKey || !release?.releaseHash || !release?.bundle || !release?.manifest) throw new Error("incomplete release");
    const active = this.active.get(release.scopeKey);
    if (active?.release.releaseHash === release.releaseHash && !active.closed) return { releaseHash: release.releaseHash, ready: true };
    if (active?.closed) this.active.delete(release.scopeKey);
    const candidateKey = this.#candidateKey(release.scopeKey, release.releaseHash);
    if (this.candidates.has(candidateKey)) return { releaseHash: release.releaseHash, ready: true };
    const worker = new ReleaseWorker(release, this.capabilityExecutor);
    try {
      await worker.ready(this.prepareTimeoutMs);
    } catch (error) {
      worker.close();
      throw error;
    }
    const previous = this.candidates.get(candidateKey);
    if (previous) previous.close();
    this.candidates.set(candidateKey, worker);
    return { releaseHash: release.releaseHash, ready: true };
  }

  activate(scopeKey, releaseHash) {
    const previous = this.active.get(scopeKey);
    if (previous?.release.releaseHash === releaseHash && !previous.closed) return { releaseHash };
    const candidateKey = this.#candidateKey(scopeKey, releaseHash);
    const candidate = this.candidates.get(candidateKey);
    if (!candidate) throw new Error("prepared release not found for scope");
    this.candidates.delete(candidateKey);
    this.active.set(scopeKey, candidate);
    if (previous && previous !== candidate) previous.drain();
    return { releaseHash };
  }

  async execute({ scopeKey, releaseHash, operationId, variables = {}, exposure = "app", scopeToken }) {
    const worker = this.active.get(scopeKey);
    if (!worker || worker.release.releaseHash !== releaseHash) throw Object.assign(new Error("release changed"), { code: "RELEASE_CHANGED" });
    const operation = worker.release.manifest.graphql?.operations?.[operationId];
    if (!operation || !operation.exposure?.includes(exposure)) throw Object.assign(new Error("persisted operation is not exposed"), { code: "FORBIDDEN" });
    // The parent must be able to terminate synchronous authored code, whose
    // event loop cannot fire an in-worker timer. Use the strictest resolver
    // budget referenced by this persisted operation as its outer bound.
    const timeout = worker.timeoutFor(operationId);
    return worker.execute({ operationId, variables, scopeToken }, timeout);
  }

  async invokeLambda({ scopeKey, releaseHash, name, input = {}, scopeToken }) {
    const worker = this.active.get(scopeKey);
    if (!worker || worker.release.releaseHash !== releaseHash) throw Object.assign(new Error("release changed"), { code: "RELEASE_CHANGED" });
    const lambda = worker.release.manifest.lambdas?.[name];
    if (!lambda || lambda.trigger?.kind !== "manual") throw Object.assign(new Error("lambda is not available"), { code: "NOT_FOUND" });
    return worker.invokeLambda({ name, input, scopeToken }, lambda.budget.timeoutMs);
  }

  remove(scopeKey, releaseHash) {
    const candidateKey = this.#candidateKey(scopeKey, releaseHash);
    const candidate = this.candidates.get(candidateKey);
    if (candidate) {
      candidate.close();
      this.candidates.delete(candidateKey);
    }
    const active = this.active.get(scopeKey);
    if (active?.release.releaseHash === releaseHash) {
      this.active.delete(scopeKey);
      active.drain();
    }
  }

  close() {
    for (const worker of [...this.candidates.values(), ...this.active.values()]) worker.close();
    this.candidates.clear();
    this.active.clear();
  }

  #candidateKey(scopeKey, releaseHash) {
    return `${scopeKey}\u0000${releaseHash}`;
  }
}
