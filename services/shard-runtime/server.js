import http from "node:http";
import { ReleaseRegistry } from "./src/registry.js";

const sharedSecret = process.env.SHARD_RUNTIME_SECRET;
if (!sharedSecret) throw new Error("SHARD_RUNTIME_SECRET is required");
const capabilityURL = process.env.SHARD_CAPABILITY_URL;
if (!capabilityURL) throw new Error("SHARD_CAPABILITY_URL is required");

const registry = new ReleaseRegistry({ capabilityExecutor: async (request) => {
  const response = await fetch(capabilityURL, { method: "POST", headers: { "content-type": "application/json", authorization: `Bearer ${sharedSecret}` }, body: JSON.stringify(request) });
  const body = await response.json();
  if (!response.ok) throw Object.assign(new Error(body.error || `capability service returned ${response.status}`), { code: body.code || "CAPABILITY_FAILED" });
  return body;
} });

const server = http.createServer(async (request, response) => {
  response.setHeader("content-type", "application/json");
  if (request.method === "GET" && request.url === "/healthz") {
    response.end(JSON.stringify({ ok: true }));
    return;
  }
  if (request.headers.authorization !== `Bearer ${sharedSecret}`) {
    response.statusCode = 401;
    response.end(JSON.stringify({ error: "unauthorized" }));
    return;
  }
  const chunks = [];
  let bytes = 0;
  for await (const chunk of request) {
    bytes += chunk.length;
    if (bytes > 16 << 20) { response.statusCode = 413; response.end(JSON.stringify({ error: "request too large" })); return; }
    chunks.push(chunk);
  }
  try {
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}");
    let result;
    if (request.url === "/v1/releases/prepare") result = await registry.prepare(body);
    else if (request.url === "/v1/releases/activate") result = registry.activate(body.scopeKey, body.releaseHash);
    else if (request.url === "/v1/releases/remove") result = registry.remove(body.scopeKey, body.releaseHash) || {};
    else if (request.url === "/v1/graphql/execute") result = await registry.execute(body);
    else if (request.url === "/v1/lambdas/invoke") result = await registry.invokeLambda(body);
    else { response.statusCode = 404; result = { error: "not found" }; }
    response.end(JSON.stringify(result));
  } catch (error) {
    response.statusCode = error.code === "FORBIDDEN" ? 403 : error.code === "RELEASE_CHANGED" ? 409 : 400;
    response.end(JSON.stringify({ code: error.code || "RUNTIME_ERROR", error: String(error.message || error) }));
  }
});

server.listen(Number(process.env.PORT || 8092), process.env.HOST || "127.0.0.1");
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => { registry.close(); server.close(); });
