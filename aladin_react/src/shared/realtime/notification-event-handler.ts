import type { AppEventEnvelope } from "@/shared/realtime/app-event";
import { useAppStore } from "@/app/state/store";
import type { NotificationView } from "@/app/state/notifications-slice";

// Routes "notification.created" workspace events (price alerts firing, extensible to other
// producers) into the notifications store → bell badge + toast. Durability is the server's
// notifications table (fetched on load); this is the best-effort live delivery.
export function createNotificationEventHandler() {
  return function handle(event: AppEventEnvelope) {
    if (event.type !== "notification.created") return;
    const p = event.payload as Record<string, unknown> | null;
    if (!p || typeof p.id !== "string") return;
    const n: NotificationView = {
      id: p.id,
      kind: typeof p.kind === "string" ? p.kind : "notification",
      title: typeof p.title === "string" ? p.title : "",
      body: typeof p.body === "string" ? p.body : "",
      data: (p.data as Record<string, unknown> | undefined) ?? undefined,
      createdAt: typeof p.createdAt === "string" ? p.createdAt : new Date().toISOString(),
      read: false,
    };
    useAppStore.getState().addNotification(n);
  };
}
