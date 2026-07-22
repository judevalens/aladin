import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import type { CopilotCitation } from "@/app/state/copilot-slice";

// Routes "copilot.*" workspace events (token/tool/message/done/error) into the copilot store.
// Each event carries the sessionId of the turn that produced it; the slice ignores events for a
// stale turn. Ephemeral UI state — never touches the offline data layer.
export function createCopilotEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (!event.type.startsWith("copilot.")) return;
    const payload = event.payload as Record<string, unknown> | null;
    const sessionId = payload && typeof payload.sessionId === "string" ? payload.sessionId : "";
    if (!sessionId) return;
    const store = useAppStore.getState();

    switch (event.type) {
      case "copilot.token": {
        const delta = typeof payload?.delta === "string" ? payload.delta : "";
        if (delta) store.appendCopilotToken(sessionId, delta);
        return;
      }
      case "copilot.tool": {
        const label =
          typeof payload?.label === "string" && payload.label
            ? payload.label
            : typeof payload?.name === "string"
              ? payload.name
              : "";
        if (label) store.setCopilotTool(sessionId, label);
        return;
      }
      case "copilot.message": {
        const id = typeof payload?.messageId === "string" ? payload.messageId : `srv-${Date.now()}`;
        const content = typeof payload?.content === "string" ? payload.content : "";
        const citations = Array.isArray(payload?.citations)
          ? (payload?.citations as CopilotCitation[])
          : [];
        store.finishCopilotMessage(sessionId, { id, role: "assistant", content, citations });
        return;
      }
      case "copilot.done":
        store.endCopilotTurn(sessionId);
        return;
      case "copilot.error": {
        const message = typeof payload?.message === "string" ? payload.message : "The assistant hit an error.";
        store.setCopilotError(sessionId, message);
        return;
      }
    }
  };
}
