import * as DialogPrimitive from "@radix-ui/react-dialog";
import { Eyebrow } from "@/components/ui/eyebrow";
import { Icon } from "@/components/ui/icon";
import { X } from "lucide-react";

import { useAppStore } from "@/app/state/store";
import { useMarketData } from "@/modules/markets/hooks/use-market-data";
import { useQuoteSubscription } from "@/modules/markets/hooks/use-quote-subscription";
import { TickerDetail } from "@/modules/markets/ui/ticker-detail";

// The inner content mounts only while open, so the watchlist fetch doesn't run otherwise.
function TickerModalBody({ symbol, onClose }: { symbol: string; onClose: () => void }) {
  const { getQuote } = useMarketData();
  useQuoteSubscription([symbol]);
  const quote = getQuote(symbol);
  return (
    <div className="flex h-[min(760px,88vh)] flex-col overflow-hidden">
      <div className="flex items-center justify-between border-b border-line px-4 py-2">
        <Eyebrow as="span" className="text-ink-4">Ticker</Eyebrow>
        <DialogPrimitive.Close asChild>
          <button type="button" aria-label="Close" className="grid size-7 place-items-center rounded text-ink-4 hover:text-ink">
            <Icon as={X} />
          </button>
        </DialogPrimitive.Close>
      </div>
      <div className="min-h-0 flex-1">
        <TickerDetail quote={quote} onTrade={onClose} />
      </div>
    </div>
  );
}

/**
 * The global ticker modal — opening a ticker anywhere (the command box, a link) shows its
 * chart + data here, rather than a full-page route. Driven by the store's openTickerSymbol.
 */
export function TickerModal() {
  const symbol = useAppStore((s) => s.openTickerSymbol);
  const closeTicker = useAppStore((s) => s.closeTicker);
  const open = symbol !== null;

  return (
    <DialogPrimitive.Root open={open} onOpenChange={(o) => !o && closeTicker()}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-scrim backdrop-blur-[2px] data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content className="fixed left-1/2 top-1/2 z-50 w-[min(560px,calc(100vw-64px))] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-modal border border-line bg-panel shadow-modal animate-pop">
          <DialogPrimitive.Title className="sr-only">Ticker detail</DialogPrimitive.Title>
          {symbol && <TickerModalBody symbol={symbol} onClose={closeTicker} />}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
