import { parentPort, workerData } from "node:worker_threads";
import { buildSchema, graphql, parse, validate } from "graphql";

const manifest = workerData.manifest;
const capabilities = new Map();
for (const [name, handler] of Object.entries(manifest.graphql?.resolvers || {})) capabilities.set(name, handler);
for (const [name, handler] of Object.entries(manifest.lambdas || {})) capabilities.set(`lambda:${name}`, handler);
const pendingCapabilities = new Map();
let moduleValue;
let schema;
let startupError;

try {
  moduleValue = await import(`data:text/javascript;base64,${Buffer.from(workerData.bundle).toString("base64")}`);
  schema = manifest.graphql ? buildSchema(workerData.schema) : undefined;
  for (const [name, operation] of Object.entries(manifest.graphql?.operations || {})) {
    const document = parse(operation.document);
    const errors = validate(schema, document);
    if (errors.length) throw new Error(`operation ${name}: ${errors[0].message}`);
  }
  for (const [name] of Object.entries(manifest.graphql?.resolvers || {})) {
    if (typeof moduleValue.resolvers?.[name] !== "function") throw new Error(`resolver ${name} does not export a function`);
  }
  for (const [name] of Object.entries(manifest.lambdas || {})) {
    if (typeof moduleValue.lambdas?.[name] !== "function") throw new Error(`lambda ${name} does not export a function`);
  }
} catch (error) {
  startupError = error;
}

function capabilityClient(handlerName, scopeToken) {
  const definition = capabilities.get(handlerName);
  let operations = 0;
  let documents = 0;
  return Object.freeze({
    async call(capability, input = {}) {
      if (!definition?.capabilities.includes(capability)) throw new Error(`capability ${capability} is not granted`);
      operations++;
      if (operations > definition.budget.maxOperations) throw new Error("resolver operation budget exceeded");
      const id = crypto.randomUUID();
      const result = await new Promise((resolve, reject) => {
        pendingCapabilities.set(id, { resolve, reject });
        parentPort.postMessage({ type: "capability", id, handler: handlerName, capability, input, scopeToken });
      });
      documents += Array.isArray(result) ? result.length
        : Array.isArray(result?.records) ? result.records.length
        : result?.record || result?.tombstone ? 1
        : Number(result?.documentsRead || 0);
      if (documents > definition.budget.maxDocuments) throw new Error("resolver document budget exceeded");
      return result;
    }
  });
}

async function resolveHandler(source, args, context, info) {
  const qualified = `${info.parentType.name}.${info.fieldName}`;
  const name = moduleValue.resolvers?.[qualified] ? qualified : info.fieldName;
  const handler = moduleValue.resolvers?.[name];
  if (!handler) return source?.[info.fieldName];
  const timeoutMs = capabilities.get(name)?.budget?.timeoutMs || 1000;
  let timer;
  try {
    return await Promise.race([
      Promise.resolve().then(() => handler(args, Object.freeze({ capabilities: capabilityClient(name, context.scopeToken), request: context.request }))),
      new Promise((_, reject) => { timer = setTimeout(() => {
        parentPort.postMessage({ type: "handler-timeout", requestId: context.request.requestId });
        reject(new Error("resolver execution timed out"));
      }, timeoutMs); })
    ]);
  } finally {
    clearTimeout(timer);
  }
}

parentPort.on("message", async (message) => {
  if (message.type === "capability-result") {
    const pending = pendingCapabilities.get(message.id);
    if (!pending) return;
    pendingCapabilities.delete(message.id);
    if (message.ok) pending.resolve(message.result); else pending.reject(Object.assign(new Error(message.error), { code: message.code || "CAPABILITY_FAILED" }));
    return;
  }
  if (message.type === "ready") {
    if (startupError) parentPort.postMessage({ type: "result", id: message.id, ok: false, error: startupError.message });
    else parentPort.postMessage({ type: "result", id: message.id, ok: true, result: { ready: true } });
    return;
  }
  try {
    if (startupError) throw startupError;
    if (message.type === "execute") {
      const operation = manifest.graphql.operations[message.operationId];
      const result = await graphql({ schema, source: operation.document, variableValues: message.variables, contextValue: { request: { operationId: message.operationId, requestId: message.id }, scopeToken: message.scopeToken }, fieldResolver: resolveHandler });
      parentPort.postMessage({ type: "result", id: message.id, ok: true, result });
    } else if (message.type === "lambda") {
      const handler = moduleValue.lambdas[message.name];
      const result = await handler(message.input, Object.freeze({ capabilities: capabilityClient(`lambda:${message.name}`, message.scopeToken) }));
      parentPort.postMessage({ type: "result", id: message.id, ok: true, result });
    }
  } catch (error) {
    parentPort.postMessage({ type: "result", id: message.id, ok: false, error: String(error?.message || error) });
  }
});
