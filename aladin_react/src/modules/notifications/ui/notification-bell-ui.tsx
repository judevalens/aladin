import { Bell, Check, X } from "lucide-react";
import { Icon } from "@/components/ui/icon";
import { useEffect } from "react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useAppStore } from "@/app/state/store";
import { useNotifications } from "@/modules/notifications/hooks/use-notifications";
import type { NotificationView } from "@/app/state/notifications-slice";
import { cn } from "@/lib/utils";

/**
 * The notification bell — rail affordance with an unread badge + a dropdown inbox. Alerts
 * firing (and any future producer) land here live via notification.created; the durable list
 * hydrates on load. Clicking a price-alert notification opens its ticker.
 */
export function NotificationBell() {
  const { notifications, unread, open, setOpen, markRead } = useNotifications();
  const openTicker = useAppStore((s) => s.openTicker);

  const onNotificationClick = (n: NotificationView) => {
    if (!n.read) markRead(n.id);
    const sym = typeof n.data?.symbol === "string" ? n.data.symbol : "";
    if (sym) openTicker(sym);
  };

  return (
    <DropdownMenu open={open} onOpenChange={setOpen}>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label="Notifications"
              className={cn(
                "relative grid size-[38px] place-items-center rounded-control transition-colors",
                open ? "bg-[rgb(var(--sel))] text-ink" : "text-ink-3 hover:bg-[rgb(var(--hover))] hover:text-ink",
              )}
            >
              <Icon as={Bell} size="rail" />
              {unread > 0 ? (
                <span className="absolute right-1 top-1 grid min-w-3.5 place-items-center rounded-full bg-amber px-1 text-meta font-semibold leading-[14px] text-primary-foreground">
                  {unread > 9 ? "9+" : unread}
                </span>
              ) : null}
            </button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="right">Notifications</TooltipContent>
      </Tooltip>

      <DropdownMenuContent align="start" side="right" className="w-80 p-0">
        <div className="flex items-center justify-between border-b border-line px-3 py-2">
          <span className="font-display text-body font-semibold text-ink">Notifications</span>
          {unread > 0 ? <span className="font-mono text-meta text-ink-4">{unread} unread</span> : null}
        </div>
        <div className="max-h-96 overflow-y-auto">
          {notifications.length === 0 ? (
            <p className="px-3 py-6 text-center text-small text-ink-4">Nothing yet.</p>
          ) : (
            notifications.map((n) => (
              <button
                key={n.id}
                type="button"
                onClick={() => onNotificationClick(n)}
                className={cn(
                  "flex w-full items-start gap-2 border-b border-line/60 px-3 py-2 text-left transition-colors last:border-0 hover:bg-raise",
                  !n.read && "bg-amber-soft/20",
                )}
              >
                {!n.read ? (
                  <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-amber" />
                ) : (
                  <Icon as={Check} size="inline" mark className="mt-1 shrink-0 text-ink-4" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="truncate text-small font-medium text-ink">{n.title}</p>
                  {n.body ? <p className="truncate text-meta text-ink-3">{n.body}</p> : null}
                </div>
              </button>
            ))
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * A transient toast raised when a notification arrives live. Auto-dismisses; mounted once.
 */
export function NotificationToast() {
  const { toast, dismissToast, markRead } = useNotifications();
  const openTicker = useAppStore((s) => s.openTicker);

  useEffect(() => {
    if (!toast) return;
    const t = window.setTimeout(dismissToast, 6000);
    return () => window.clearTimeout(t);
  }, [toast, dismissToast]);

  if (!toast) return null;
  const sym = typeof toast.data?.symbol === "string" ? toast.data.symbol : "";

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-50">
      <div className="pointer-events-auto flex w-80 items-start gap-2.5 rounded-card border border-amber-line bg-panel p-3 shadow-toast">
        <Icon as={Bell} className="mt-0.5 shrink-0 text-amber" />
        <button
          type="button"
          onClick={() => {
            markRead(toast.id);
            if (sym) openTicker(sym);
            dismissToast();
          }}
          className="min-w-0 flex-1 text-left"
        >
          <p className="truncate text-body font-semibold text-ink">{toast.title}</p>
          {toast.body ? <p className="truncate text-meta text-ink-3">{toast.body}</p> : null}
        </button>
        <button
          type="button"
          onClick={dismissToast}
          aria-label="Dismiss"
          className="text-ink-4 hover:text-ink"
        >
          <Icon as={X} size="inline" mark />
        </button>
      </div>
    </div>
  );
}
