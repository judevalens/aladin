import { useBoardHost } from "./board-host";
import { useBoardToasts } from "./board-toasts";

export function boardSourceUrl(raw: string): string | null {
  try {
    const url = new URL(raw);
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password) return null;
    return url.href;
  } catch { return null; }
}

/** One opening path for the card icon and selection action. Invoke synchronously from
 * the click so browser fallback keeps user activation; never navigate the board itself. */
export function useOpenBoardSource() {
  const host = useBoardHost();
  const toasts = useBoardToasts();
  return (raw: string) => {
    const url = boardSourceUrl(raw);
    if (!url) { toasts.show({ text: "This source needs a valid http or https link." }); return; }
    try {
      if (host.onOpenExternalUrl) {
        void Promise.resolve(host.onOpenExternalUrl(url)).catch(() => {
          toasts.show({ text: "Couldn't open this source. Try again after updating the app." });
        });
      } else {
        window.open(url, "_blank", "noopener,noreferrer");
      }
    } catch {
      toasts.show({ text: "Couldn't open this source. Check your browser settings and try again." });
    }
  };
}
