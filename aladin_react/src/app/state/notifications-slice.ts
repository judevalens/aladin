import type { StateCreator } from "zustand";

/** One durable inbox item (price alert firing, extensible to other producers). */
export interface NotificationView {
  id: string;
  kind: string;
  title: string;
  body: string;
  data?: Record<string, unknown>;
  createdAt: string;
  read: boolean;
}

export interface NotificationsSlice {
  notifications: NotificationView[];
  /** True while the bell panel is open. */
  notificationsOpen: boolean;
  /** Most recent unacknowledged toast (or null). */
  notificationToast: NotificationView | null;

  toggleNotifications: () => void;
  setNotificationsOpen: (open: boolean) => void;
  /** Replace the list from a fresh fetch (hydrate on load — the offline-survival read path). */
  setNotifications: (items: NotificationView[]) => void;
  /** A live notification.created event arrived: prepend it + raise a toast. */
  addNotification: (n: NotificationView) => void;
  /** Locally mark one read (optimistic; the POST confirms). */
  markNotificationRead: (id: string) => void;
  dismissNotificationToast: () => void;
}

const unread = (items: NotificationView[]) => items.filter((n) => !n.read).length;

export const createNotificationsSlice: StateCreator<NotificationsSlice, [], [], NotificationsSlice> = (set) => ({
  notifications: [],
  notificationsOpen: false,
  notificationToast: null,

  toggleNotifications: () => set((s) => ({ notificationsOpen: !s.notificationsOpen })),
  setNotificationsOpen: (open) => set({ notificationsOpen: open }),

  setNotifications: (items) => set({ notifications: items }),

  addNotification: (n) =>
    set((s) => {
      if (s.notifications.some((x) => x.id === n.id)) return {}; // de-dupe (reconnect replays)
      return { notifications: [n, ...s.notifications], notificationToast: n };
    }),

  markNotificationRead: (id) =>
    set((s) => ({
      notifications: s.notifications.map((n) => (n.id === id ? { ...n, read: true } : n)),
    })),

  dismissNotificationToast: () => set({ notificationToast: null }),
});

/** Derived selector: the unread count for the bell badge. */
export const selectUnreadCount = (items: NotificationView[]) => unread(items);
