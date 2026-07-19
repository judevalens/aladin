// Placeholder market data. DETERMINISTIC per symbol so the UI is stable (no flicker) while
// there's no live feed. This is the seam for Alpaca: `Quote` is the shape the reactive feed
// will emit (event-driven via the outbox → DataEvents → RxJS), and every consumer already
// reads it — so wiring real data is a data-source swap, not a UI change.

export interface Quote {
  symbol: string;
  name: string;
  sector: string;
  last: number;
  change: number; // absolute, vs prev close
  changePct: number;
  open: number;
  high: number;
  low: number;
  prevClose: number;
  volume: number; // shares
  marketCap: number; // dollars
  week52Low: number;
  week52High: number;
  spark: number[]; // intraday points, oldest → newest
}

export interface IndexQuote {
  label: string;
  value: number;
  changePct: number;
}

interface SymbolMeta {
  name: string;
  sector: string;
  price: number; // baseline last price
  cap: number; // baseline market cap ($)
}

// The known universe (name · sector · rough price/cap). Unknown symbols fall back to a
// generic tech profile so search-added tickers still render.
const SYMBOL_META: Record<string, SymbolMeta> = {
  NVDA: { name: "NVIDIA", sector: "Technology", price: 1183.56, cap: 3_400_000_000_000 },
  AAPL: { name: "Apple", sector: "Technology", price: 227.14, cap: 3_450_000_000_000 },
  MSFT: { name: "Microsoft", sector: "Technology", price: 481.18, cap: 3_580_000_000_000 },
  AMD: { name: "AMD", sector: "Technology", price: 168.4, cap: 272_000_000_000 },
  AVGO: { name: "Broadcom", sector: "Technology", price: 172.9, cap: 800_000_000_000 },
  ORCL: { name: "Oracle", sector: "Technology", price: 205.67, cap: 570_000_000_000 },
  META: { name: "Meta Platforms", sector: "Communication", price: 609.14, cap: 1_540_000_000_000 },
  GOOGL: { name: "Alphabet", sector: "Communication", price: 195.37, cap: 2_390_000_000_000 },
  GOOG: { name: "Alphabet", sector: "Communication", price: 196.9, cap: 2_390_000_000_000 },
  NFLX: { name: "Netflix", sector: "Communication", price: 712.3, cap: 305_000_000_000 },
  AMZN: { name: "Amazon", sector: "Consumer", price: 222.99, cap: 2_320_000_000_000 },
  TSLA: { name: "Tesla", sector: "Consumer", price: 246.2, cap: 785_000_000_000 },
  COST: { name: "Costco", sector: "Consumer", price: 915.4, cap: 405_000_000_000 },
  HD: { name: "Home Depot", sector: "Consumer", price: 402.6, cap: 400_000_000_000 },
  JPM: { name: "JPMorgan Chase", sector: "Financials", price: 248.7, cap: 700_000_000_000 },
  BAC: { name: "Bank of America", sector: "Financials", price: 46.1, cap: 355_000_000_000 },
  V: { name: "Visa", sector: "Financials", price: 294.92, cap: 590_000_000_000 },
  MA: { name: "Mastercard", sector: "Financials", price: 512.4, cap: 470_000_000_000 },
  GS: { name: "Goldman Sachs", sector: "Financials", price: 516.07, cap: 165_000_000_000 },
  LLY: { name: "Eli Lilly", sector: "Health", price: 898.97, cap: 810_000_000_000 },
  UNH: { name: "UnitedHealth", sector: "Health", price: 512.1, cap: 470_000_000_000 },
  JNJ: { name: "Johnson & Johnson", sector: "Health", price: 152.3, cap: 370_000_000_000 },
  XOM: { name: "Exxon Mobil", sector: "Energy", price: 118.4, cap: 520_000_000_000 },
  SPY: { name: "SPDR S&P 500", sector: "ETF", price: 612.85, cap: 600_000_000_000 },
  QQQ: { name: "Invesco QQQ", sector: "ETF", price: 520.06, cap: 320_000_000_000 },
};

// Small deterministic string hash → [0,1). Stable across renders; distinct per symbol.
function hash01(s: string, salt = 0): number {
  let h = 2166136261 ^ salt;
  for (let i = 0; i < s.length; i++) {
    h = Math.imul(h ^ s.charCodeAt(i), 16777619);
  }
  // to unsigned, then to [0,1)
  return ((h >>> 0) % 100000) / 100000;
}

function metaFor(symbol: string): SymbolMeta {
  return (
    SYMBOL_META[symbol] ?? {
      name: symbol,
      sector: "Technology",
      price: 40 + hash01(symbol, 7) * 400,
      cap: 20_000_000_000 + hash01(symbol, 11) * 200_000_000_000,
    }
  );
}

// buildQuote synthesizes a full deterministic quote for a symbol.
export function buildQuote(symbol: string, nameOverride?: string): Quote {
  const meta = metaFor(symbol);
  const last = meta.price;
  // Day change in [-5%, +5%], deterministic per symbol.
  const changePct = (hash01(symbol, 3) - 0.5) * 10;
  const prevClose = last / (1 + changePct / 100);
  const change = last - prevClose;
  const swing = last * 0.02;
  const open = prevClose + (hash01(symbol, 5) - 0.5) * swing;
  const high = Math.max(last, open) + hash01(symbol, 9) * swing;
  const low = Math.min(last, open) - hash01(symbol, 13) * swing;
  const volume = Math.round((5 + hash01(symbol, 17) * 30) * 1_000_000);
  const week52Low = last * (0.62 + hash01(symbol, 19) * 0.1);
  const week52High = last * (1.05 + hash01(symbol, 23) * 0.2);

  // Sparkline: a random-ish walk anchored to trend toward `last`, 32 points.
  const n = 32;
  const spark: number[] = [];
  let v = prevClose;
  for (let i = 0; i < n; i++) {
    const drift = (last - prevClose) / n;
    const noise = (hash01(symbol, 100 + i) - 0.5) * swing * 0.9;
    v = v + drift + noise;
    spark.push(v);
  }
  spark[spark.length - 1] = last;

  return {
    symbol,
    name: nameOverride ?? meta.name,
    sector: meta.sector,
    last,
    change,
    changePct,
    open,
    high,
    low,
    prevClose,
    volume,
    marketCap: meta.cap,
    week52Low,
    week52High,
    spark,
  };
}

export const TIMEFRAMES = ["1D", "1W", "1M", "6M", "1Y"] as const;
export type Timeframe = (typeof TIMEFRAMES)[number];

// A deterministic price series for a timeframe (1D reuses the intraday spark). Longer
// frames are a random-ish walk ending at `last`, more points + wider amplitude.
export function seriesFor(q: Quote, tf: Timeframe): number[] {
  if (tf === "1D") return q.spark;
  const points = { "1W": 40, "1M": 60, "6M": 120, "1Y": 180 }[tf] ?? 60;
  const amp = { "1W": 0.05, "1M": 0.1, "6M": 0.25, "1Y": 0.4 }[tf] ?? 0.1;
  const start = q.last * (1 - (hash01(q.symbol, tf.length * 31) - 0.35) * amp);
  const out: number[] = [];
  let v = start;
  for (let i = 0; i < points; i++) {
    const drift = (q.last - start) / points;
    const noise = (hash01(q.symbol, 500 + i + tf.length * 7) - 0.5) * q.last * amp * 0.12;
    v = v + drift + noise;
    out.push(v);
  }
  out[out.length - 1] = q.last;
  return out;
}

// The index strip. Deterministic placeholder values matching the reference layout.
export const INDICES: IndexQuote[] = [
  { label: "S&P 500", value: 6128.53, changePct: 1.28 },
  { label: "Nasdaq 100", value: 520.06, changePct: -1.18 },
  { label: "Dow 30", value: 449.48, changePct: 0.14 },
  { label: "Russell 2000", value: 227.38, changePct: -1.67 },
  { label: "Volatility", value: 14.2, changePct: -3.4 },
];

// A default universe when the watchlist is empty, so Markets always renders something.
export const DEFAULT_UNIVERSE = [
  "NVDA", "AAPL", "MSFT", "AMD", "META", "AMZN", "TSLA", "JPM", "LLY", "V", "GS",
];

// Fake per-symbol headlines for the detail panel's "Latest on …".
export function headlinesFor(symbol: string): { source: string; title: string; ago: string }[] {
  return [
    { source: "S3 Partners", title: `Short interest in ${symbol} ticks up for a third week`, ago: "2h" },
    { source: "Barron's", title: `${symbol} added to a high-conviction model portfolio`, ago: "3h" },
    { source: "Reuters", title: `${symbol} guidance raise pulls the sector higher`, ago: "5h" },
  ];
}

// Formatting helpers.
export const fmtPrice = (n: number) =>
  n.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
export const fmtPct = (n: number) => `${n >= 0 ? "+" : ""}${n.toFixed(2)}%`;
export const fmtSigned = (n: number) => `${n >= 0 ? "+" : ""}${fmtPrice(n)}`;
export function fmtVol(n: number): string {
  if (n >= 1_000_000_000) return `${(n / 1_000_000_000).toFixed(1)}B`;
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return `${n}`;
}
export function fmtCap(n: number): string {
  if (n >= 1_000_000_000_000) return `$${(n / 1_000_000_000_000).toFixed(2)}T`;
  if (n >= 1_000_000_000) return `$${(n / 1_000_000_000).toFixed(1)}B`;
  return `$${fmtVol(n)}`;
}
