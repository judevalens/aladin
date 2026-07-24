import { useCallback, useEffect } from "react";
import { useAppComposition } from "@/app/composition/app-composition";
import { useAppStore } from "@/app/state/store";

/**
 * Binds the notification surface to its repo + store: hydrates the durable inbox on mount
 * (the offline-survival read path — realtime is best-effort), and exposes read/dismiss.
 */
export function useNotifications() {
  const { repos } = useAppComposition();
  const notifications = useAppStore((s) => s.notifications);
  const open = useAppStore((s) => s.notificationsOpen);
  const toast = useAppStore((s) => s.notificationToast);

  useEffect(() => {
    repos.notifications
      .list()
      .then((items) => useAppStore.getState().setNotifications(items))
      .catch(() => {
        // non-fatal — live events still populate the list
      });
  }, [repos.notifications]);

  const markRead = useCallback(
    (id: string) => {
      useAppStore.getState().markNotificationRead(id);
      void repos.notifications.markRead(id).catch(() => {});
    },
    [repos.notifications],
  );

  const setOpen = useCallback((next: boolean) => useAppStore.getState().setNotificationsOpen(next), []);
  const dismissToast = useCallback(() => useAppStore.getState().dismissNotificationToast(), []);

  const unread = notifications.filter((n) => !n.read).length;

  return { notifications, unread, open, setOpen, toast, markRead, dismissToast };
}
