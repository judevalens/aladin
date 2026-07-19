import { useEffect } from "react";
import { Navigate, useParams } from "react-router-dom";

import { useAppStore } from "@/app/state/store";

/**
 * The /ticker/:symbol deep-link. Tickers open as a MODAL (design decision), not a page — so
 * this route opens the global ticker modal for the symbol and redirects to the Markets
 * surface, which the modal floats over.
 */
export function TickerRoute() {
  const { symbol = "" } = useParams();
  const openTicker = useAppStore((s) => s.openTicker);

  useEffect(() => {
    if (symbol) openTicker(symbol.toUpperCase());
  }, [symbol, openTicker]);

  return <Navigate to="/markets" replace />;
}
