import type { ApiClient } from "@/shared/api/client";
import type { NotificationView } from "@/app/state/notifications-slice";

/** Wire shape from GET /api/notifications. */
interface NotificationWire {
  id: string;
  kind: string;
  title: string;
  body: string;
  data?: Record<string, unknown> | null;
  readAt?: string | null;
  createdAt: string;
}

export interface AlertView {
  id: string;
  symbol: string;
  direction: string;
  threshold: number;
  armed: boolean;
  status: string;
  lastFiredAt?: string;
  lastFiredPrice?: number;
  createdAt: string;
}

export interface CreateAlertRequest {
  symbol: string;
  direction: "above" | "below";
  threshold: number;
}

export interface NotificationsRepo {
  list(): Promise<NotificationView[]>;
  markRead(id: string): Promise<void>;
  // Alerts management (creation is usually conversational via the copilot, but exposed for a UI).
  listAlerts(): Promise<AlertView[]>;
  createAlert(req: CreateAlertRequest): Promise<{ alert: AlertView; warning?: string }>;
  deleteAlert(id: string): Promise<void>;
}

function toView(n: NotificationWire): NotificationView {
  return {
    id: n.id,
    kind: n.kind,
    title: n.title,
    body: n.body,
    data: n.data ?? undefined,
    createdAt: n.createdAt,
    read: Boolean(n.readAt),
  };
}

export function createNotificationsRepo(client: ApiClient): NotificationsRepo {
  return {
    list: () =>
      client
        .fetch<{ notifications: NotificationWire[] | null }>("/api/notifications")
        .then((r) => (r.notifications ?? []).map(toView)),

    markRead: (id) =>
      client
        .fetch<{ ok: boolean }>(`/api/notifications/${encodeURIComponent(id)}/read`, { method: "POST" })
        .then(() => undefined),

    listAlerts: () =>
      client.fetch<{ alerts: AlertView[] | null }>("/api/alerts").then((r) => r.alerts ?? []),

    createAlert: (req) =>
      client.fetch<{ alert: AlertView; warning?: string }>("/api/alerts", {
        method: "POST",
        body: JSON.stringify(req),
      }),

    deleteAlert: (id) =>
      client
        .fetch<{ ok: boolean }>(`/api/alerts/${encodeURIComponent(id)}`, { method: "DELETE" })
        .then(() => undefined),
  };
}
