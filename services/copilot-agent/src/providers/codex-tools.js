import { createServer } from "node:http";
import { randomUUID } from "node:crypto";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { config } from "../config.js";
import { makeCanUseTool } from "../approvals.js";

// One authenticated loopback bridge per turn. Codex never receives the user's
// bearer and cannot bypass the same approval gate used by the Claude adapter.
export async function createCodexTools(body, turn, emit, deps = {}) {
  const upstream = deps.mcpClient ?? new Client({ name: "aladin-copilot", version: "0.1.0" });
  const signal = turn.abortController.signal;
  const timeout = deps.turnTimeoutMs ?? config.turnTimeoutMs;
  const token = randomUUID();
  const servers = new Set();
  let listener;
  let closed = false;
  const close = async () => {
    if (closed) return;
    closed = true;
    if (listener) {
      listener.closeAllConnections();
      await new Promise((resolve) => listener.close(resolve));
    }
    await Promise.allSettled([...servers].map((server) => server.close()));
    await upstream.close();
  };

  try {
    await upstream.connect(new StreamableHTTPClientTransport(new URL(body.mcpUrl || config.mcpUrl), {
      requestInit: { headers: { Authorization: `Bearer ${body.userBearer}` } },
    }), { signal, timeout: 10_000 });
    const tools = [];
    let cursor;
    do {
      const page = await upstream.listTools(cursor ? { cursor } : {}, { signal, timeout: 10_000 });
      tools.push(...page.tools);
      cursor = page.nextCursor;
    } while (cursor);
    if (!tools.length) throw new Error("Aladin MCP returned no tools.");
    signal.throwIfAborted();
    const toolNames = new Set(tools.map((tool) => tool.name));
    const canUseTool = makeCanUseTool({
      turn, emit, gatedTools: body.gatedTools,
      timeoutMs: deps.approvalTimeoutMs ?? config.approvalTimeoutMs,
    });

    listener = createServer(async (req, res) => {
      if (req.url !== "/mcp" || req.headers.authorization !== `Bearer ${token}`) {
        res.writeHead(401).end();
        return;
      }
      if (req.method !== "POST") {
        res.writeHead(405).end();
        return;
      }
      const server = new Server({ name: "aladin", version: "0.1.0" }, { capabilities: { tools: {} } });
      servers.add(server);
      res.on("close", () => {
        servers.delete(server);
        void server.close();
      });
      server.setRequestHandler(ListToolsRequestSchema, () => ({ tools }));
      server.setRequestHandler(CallToolRequestSchema, async ({ params }) => {
        const toolUseID = randomUUID();
        let approvalId;
        let result;
        try {
          signal.throwIfAborted();
          if (!toolNames.has(params.name)) throw new Error(`Unknown Aladin tool: ${params.name}`);
          const decision = await canUseTool(params.name, params.arguments ?? {}, { toolUseID });
          approvalId = turn.approvalByToolUse.get(toolUseID);
          signal.throwIfAborted();
          if (decision.behavior !== "allow") throw new Error(decision.message);
          result = await upstream.callTool(params, undefined, { signal, timeout });
        } catch (error) {
          result = { isError: true, content: [{ type: "text", text: error.message }] };
        } finally {
          turn.approvalByToolUse.delete(toolUseID);
        }
        if (approvalId) result = { ...result, _meta: { ...result._meta, "aladin/approvalId": approvalId } };
        return result;
      });
      try {
        const transport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined, enableJsonResponse: true });
        await server.connect(transport);
        await transport.handleRequest(req, res);
      } catch {
        if (!res.headersSent) res.writeHead(500).end();
        else res.end();
        servers.delete(server);
        await server.close();
      }
    });
    await new Promise((resolve, reject) => {
      listener.once("error", reject);
      listener.listen(0, "127.0.0.1", resolve);
    });
    return { url: `http://127.0.0.1:${listener.address().port}/mcp`, token, tools, close };
  } catch (error) {
    await close();
    const status = Number.isInteger(error.code) && error.code >= 400 && error.code <= 599 ? ` (HTTP ${error.code})` : "";
    throw new Error(`The copilot's Aladin MCP tools are unavailable${status}: ${error.message}`);
  }
}
