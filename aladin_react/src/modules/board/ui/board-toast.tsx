import { useBoardToasts, useCurrentToast } from "../domain/board-toasts";

/** The one transient message, above the dock. Tap the action or wait; it leaves on its own. */
export function BoardToastView() {
  const store = useBoardToasts();
  const toast = useCurrentToast();
  if (!toast) return null;
  return (
    <div
      role="status"
      className="board-toast board-island board-island--popover board-edge-above-dock pointer-events-auto absolute left-1/2 flex h-12 -translate-x-1/2 items-center gap-3 pl-4 pr-1.5 text-board-row text-ink"
    >
      <span className="whitespace-nowrap">{toast.text}</span>
      {toast.action ? (
        <button
          type="button"
          onClick={() => {
            toast.action?.onPress();
            store.dismiss(toast.id);
          }}
          className="board-tile h-11 rounded-control px-3 font-semibold text-amber hover:bg-hover"
        >
          {toast.action.label}
        </button>
      ) : null}
    </div>
  );
}
