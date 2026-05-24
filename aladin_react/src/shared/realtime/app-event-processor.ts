import type { AppEventEnvelope } from "@/shared/realtime/app-event";

export type AppEventHandler = (event: AppEventEnvelope) => void | Promise<void>;

export interface AppEventProcessor {
  register(handler: AppEventHandler): () => void;
  dispatch(event: AppEventEnvelope): void;
}

const RECENT_EVENT_IDS_LIMIT = 256;

export function createAppEventProcessor(): AppEventProcessor {
  const handlers = new Set<AppEventHandler>();
  const recentEventIds: string[] = [];
  const recentEventIdSet = new Set<string>();

  function markSeen(eventId: string) {
    if (recentEventIdSet.has(eventId)) return false;
    recentEventIds.push(eventId);
    recentEventIdSet.add(eventId);
    if (recentEventIds.length > RECENT_EVENT_IDS_LIMIT) {
      const dropped = recentEventIds.shift();
      if (dropped) recentEventIdSet.delete(dropped);
    }
    return true;
  }

  return {
    register(handler) {
      handlers.add(handler);
      return () => {
        handlers.delete(handler);
      };
    },
    dispatch(event) {
      if (!markSeen(event.eventId)) return;
      for (const handler of handlers) {
        try {
          void handler(event);
        } catch {
          // handlers should be defensive; swallow to avoid breaking dispatch
        }
      }
    },
  };
}
