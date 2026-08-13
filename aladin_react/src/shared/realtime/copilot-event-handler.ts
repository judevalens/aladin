import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import type { CopilotCitation, CopilotMessageMeta } from "@/app/state/copilot-slice";

// Routes "copilot.*" workspace events (token/tool/message/done/error) into the copilot store.
// Each event carries the sessionId of the turn that produced it; the slice ignores events for a
// stale turn. Ephemeral UI state — never touches the offline data layer.
export function createCopilotEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (!event.type.startsWith("copilot.")) return;
    const payload = event.payload as Record<string, unknown> | null;
    const store = useAppStore.getState();

    // action_result resolves a proposal by actionId — it carries no sessionId, so handle it
    // before the session gate below.
    if (event.type === "copilot.action_result") {
      const actionId = typeof payload?.actionId === "string" ? payload.actionId : "";
      const ok = payload?.ok === true;
      const message = typeof payload?.message === "string" ? payload.message : "";
      if (actionId) store.resolveCopilotProposal(actionId, ok, message);
      return;
    }

	    const sessionId = payload && typeof payload.sessionId === "string" ? payload.sessionId : "";
	    if (!sessionId) return;
	    const threadId = payload && typeof payload.threadId === "string" ? payload.threadId : "";

	    switch (event.type) {
	      case "copilot.token": {
	        const delta = typeof payload?.delta === "string" ? payload.delta : "";
	        if (delta) store.appendCopilotToken(threadId, sessionId, delta);
	        return;
	      }
	      case "copilot.tool": {
        const name = typeof payload?.name === "string" ? payload.name : "";
	        const label =
	          typeof payload?.label === "string" && payload.label ? payload.label : name;
	        if (label) store.setCopilotTool(threadId, sessionId, name || label, label);
	        return;
	      }
	      case "copilot.tool_done": {
	        const name = typeof payload?.name === "string" ? payload.name : "";
	        if (name) store.finishCopilotTool(threadId, sessionId, name, payload?.ok === true);
	        return;
	      }
	      case "copilot.thinking":
	        store.setCopilotThinking(threadId, sessionId);
	        return;
      case "copilot.message": {
        const id = typeof payload?.messageId === "string" ? payload.messageId : `srv-${Date.now()}`;
        const content = typeof payload?.content === "string" ? payload.content : "";
        const citations = Array.isArray(payload?.citations)
          ? (payload?.citations as CopilotCitation[])
          : [];
        const meta =
          payload?.meta && typeof payload.meta === "object"
            ? (payload.meta as CopilotMessageMeta)
            : undefined;
	        store.finishCopilotMessage(threadId, sessionId, { id, role: "assistant", content, citations, meta });
	        return;
	      }
	      case "copilot.proposed_action": {
	        const actionId = typeof payload?.actionId === "string" ? payload.actionId : "";
	        const tool = typeof payload?.tool === "string" ? payload.tool : "";
	        const summary = typeof payload?.summary === "string" ? payload.summary : tool;
	        if (actionId) store.addCopilotProposal({ actionId, threadId, sessionId, tool, summary });
	        return;
	      }
	      case "copilot.done":
	        store.endCopilotTurn(threadId, sessionId);
	        return;
	      case "copilot.error": {
	        const message = typeof payload?.message === "string" ? payload.message : "The assistant hit an error.";
	        const code = typeof payload?.code === "string" ? payload.code : undefined;
	        store.setCopilotError(threadId, sessionId, message, code);
	        return;
	      }
    }
  };
}
