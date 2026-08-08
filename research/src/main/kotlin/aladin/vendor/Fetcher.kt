package aladin.vendor

import aladin.BarRow
import aladin.DateRange
import aladin.Instrument
import aladin.Schema

/**
 * A source of bars.
 *
 * Batched on purpose: vendors accept many symbols per request and bill by bytes, so a
 * universe backfill should be a handful of calls rather than thousands.
 */
interface BarFetcher {
    /**
     * The data SCOPE this fetcher speaks for — `"databento:EQUS.SUMMARY"`, not
     * `"databento"`.
     *
     * Datasets from one vendor answer different questions. On 2024-08-01 AAPL is 218.36
     * on 62,500,996 shares consolidated, and 219.65 on 21,277,576 from XNAS.ITCH. Both
     * correct; neither substitutes for the other, and nothing in the numbers says which
     * you are holding. Keying on the vendor would collide them on the ohlcv primary key
     * and silently keep whichever landed first.
     */
    val source: String

    /** True when the vendor returns split/dividend-adjusted prices. Recorded, never assumed. */
    val adjusted: Boolean get() = false

    /**
     * The span this source can actually serve, or null when unknown/unbounded.
     *
     * Asking outside it is not an error the caller made so much as one the store should
     * not have made on their behalf: without this, requesting a range that predates the
     * dataset costs a round trip and then throws, halfway through a multi-gap fetch.
     */
    val availability: DateRange? get() = null

    fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow>
}

/** A fetcher that can price a request before making it. */
interface PricedFetcher : BarFetcher {
    fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange): Double

    /** The vendor's own record count — the guard against silent truncation. */
    fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange): Long
}

/**
 * Resolves a ticker's full validity history.
 *
 * One call per symbol answers every later as-of question, including the ones that
 * correctly return nothing — which is the case that matters for delisted names.
 */
interface SymbologySource {
    val source: String

    /** Every interval this ticker denoted an instrument. Empty means the vendor knows nothing. */
    fun history(symbol: String): List<Instrument>
}

/**
 * Serialises fetching only.
 *
 * **Not sufficient on its own** — the coverage write happens after the fetch returns, so
 * two threads can still collide on it. `ensureBars` holds the real lock; this exists for
 * callers driving a fetcher directly.
 */
class LockedFetcher(private val delegate: BarFetcher) : BarFetcher by delegate {
    private val lock = Any()
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> =
        synchronized(lock) { delegate.fetch(instruments, schema, range) }
}
