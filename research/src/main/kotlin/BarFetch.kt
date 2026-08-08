/**
 * Read-through bar store: ask for a range, get it — fetching only what's missing.
 *
 * The whole point of the coverage ledger is that a historical range is paid for once,
 * so the store, not the caller, decides whether a fetch happens.
 */

import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.sql.Connection
import java.sql.Timestamp
import java.time.DayOfWeek
import java.time.LocalDate
import java.time.LocalDateTime
import java.net.URLEncoder
import java.util.Base64
import kotlin.text.Charsets.UTF_8

data class BarRow(
    /** TIMESTAMP, not DATE — the same table holds 1-min and 5-min bars. */
    val ts: LocalDateTime,
    val instrumentId: Long,
    val open: Double?, val high: Double?, val low: Double?, val close: Double?,
    val volume: Long?,
)

interface BarFetcher {
    /**
     * Identifies the DATA SCOPE, not the vendor — e.g. "databento:EQUS.SUMMARY", not
     * "databento". Datasets from one vendor answer different questions: EQUS.SUMMARY
     * is the consolidated tape (AAPL 2024-08-01: 62.5M shares) while XNAS.ITCH is
     * Nasdaq only (21.3M). Both are correct; neither substitutes for the other.
     *
     * Keying on the vendor would collide them on the ohlcv primary key, and
     * ON CONFLICT DO NOTHING would silently keep whichever landed first. It also
     * makes coverage per-scope, so holding EQUS.SUMMARY never satisfies a request
     * for XNAS.ITCH.
     */
    val source: String

    /** True when the vendor returns split/dividend-adjusted prices. */
    val adjusted: Boolean get() = false

    /**
     * Batched on purpose: vendors accept many symbols per request and bill by bytes,
     * so a universe backfill should be a handful of calls, not thousands.
     */
    fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow>
}

/**
 * Serialises fetching only. **Not sufficient on its own** — the coverage write happens
 * after the fetch returns, so two threads can still collide on it. [ensureBars] holds
 * the real lock; this exists for callers driving a fetcher directly.
 */
class LockedFetcher(private val delegate: BarFetcher) : BarFetcher by delegate {
    private val lock = Any()
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange) =
        synchronized(lock) { delegate.fetch(instruments, schema, range) }
}

/**
 * Guards the whole read-through cycle — gap computation, fetch, bar insert, coverage
 * write. Locking only the fetch is not enough: DuckDB's MVCC rejects the second of two
 * concurrent DELETE+INSERTs on the same coverage row ("Conflict on tuple deletion"),
 * and the fetch has already been paid for by then.
 *
 * A single global lock is right here — DuckDB is single-writer and the engine is one
 * JVM, so per-slice granularity would buy nothing.
 */
private val ensureLock = Any()

fun Connection.createOhlcvTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS ohlcv (
          source        VARCHAR   NOT NULL,
          instrument_id BIGINT    NOT NULL,
          schema        VARCHAR   NOT NULL,
          ts            TIMESTAMP NOT NULL,             -- TIMESTAMP: holds intraday too
          open DOUBLE, high DOUBLE, low DOUBLE, close DOUBLE,   -- nullable: a halt is not 0.0
          volume   BIGINT,
          adjusted BOOLEAN NOT NULL,                    -- raw vs vendor-adjusted; never assume
          -- coverage should prevent duplicates, but it must not be the ONLY thing that
          -- does: doubled bars produce plausible wrong numbers rather than errors.
          PRIMARY KEY (source, instrument_id, schema, ts)
        )""")
}

/**
 * The last session that is definitely complete. Today's bar is partial until the close,
 * so caching it would freeze an incomplete day forever.
 *
 * Weekend-aware only — a real trading calendar (holidays, half days) is still owed.
 */
fun lastSettledSession(today: LocalDate = LocalDate.now()): LocalDate {
    var d = today.minusDays(1)
    while (d.dayOfWeek == DayOfWeek.SATURDAY || d.dayOfWeek == DayOfWeek.SUNDAY) d = d.minusDays(1)
    return d
}

/**
 * Ensure [range] is held for every instrument in [instruments], fetching only the gaps.
 * Returns bars newly fetched — 0 means it was already a full hit.
 *
 * Gaps are computed per instrument, then grouped so instruments sharing a gap go out in
 * one vendor request.
 */
fun Connection.ensureBars(
    fetcher: BarFetcher,
    instruments: Map<String, Long>,
    schema: Schema,
    range: DateRange,
): Long = synchronized(ensureLock) {
    val settled = lastSettledSession()
    if (range.from.isAfter(settled)) return@synchronized 0
    val want = DateRange(range.from, minOf(range.to, settled))

    val bySymbol = instruments.entries.flatMap { (sym, id) ->
        missingRanges(Slice(fetcher.source, id, schema), want).map { gap -> gap to (sym to id) }
    }
    if (bySymbol.isEmpty()) return@synchronized 0

    var fetched = 0L
    for ((gap, group) in bySymbol.groupBy({ it.first }, { it.second })) {
        val batch = group.toMap()
        val rows = fetcher.fetch(batch, schema, gap)

        // Bars and coverage must land together. A crash between them would leave the
        // ledger claiming data the store lacks — the one unrecoverable failure here,
        // since nothing would ever re-fetch it.
        val restore = autoCommit
        autoCommit = false
        try {
            prepareStatement(
                "INSERT INTO ohlcv VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING"
            ).use { ps ->
                for (b in rows) {
                    ps.setString(1, fetcher.source); ps.setLong(2, b.instrumentId); ps.setString(3, schema.wire)
                    ps.setTimestamp(4, Timestamp.valueOf(b.ts))
                    ps.setObject(5, b.open); ps.setObject(6, b.high)
                    ps.setObject(7, b.low); ps.setObject(8, b.close); ps.setObject(9, b.volume)
                    ps.setBoolean(10, fetcher.adjusted)
                    ps.addBatch()
                }
                ps.executeBatch()
            }
            for (id in batch.values) recordCoverage(Slice(fetcher.source, id, schema), gap)
            commit()
        } catch (e: Exception) {
            rollback(); throw e
        } finally {
            autoCommit = restore
        }
        fetched += rows.size
    }
    return@synchronized fetched
}

// ---------------------------------------------------------------------------
// Databento.
//
// Verified against real responses (the published reference would not load):
//   - `pretty_px=true` returns decimal prices and `pretty_ts=true` ISO-8601
//     timestamps. Without them prices are int64 scaled by 1e-9 and timestamps are
//     nanoseconds-since-epoch, both of which have to be decoded by hand — a wrong
//     scale would silently corrupt every price, so let the server format.
//   - `stype_in=raw_symbol` or tickers are read as vendor instrument ids.
//   - `map_symbols=true` or there is no `symbol` column and batched rows cannot be
//     attributed back.
//   - `end` is EXCLUSIVE; start == end is a 422.
//   - Row order is NOT stable across requests, so rows are keyed by symbol.
//
// From the official Python client (the authoritative contract — the published
// reference would not load):
//   - the endpoint is POST with FORM-ENCODED parameters, not GET. At 2,000 symbols
//     a query string would exceed URL length limits.
//   - **max 2,000 symbols per request** — chunked below.
//   - there is no pagination; `limit` is opt-in and otherwise the full result
//     streams. The record-count assertion below is therefore cheap insurance rather
//     than a necessity, but silent truncation is the one failure that looks exactly
//     like missing data, so it stays.
//   - DBN+zstd are the real defaults; CSV is a convenience encoding. Switching to
//     DBN would need a decoder and is worth it only once volume justifies it.
//
// Dataset choice is not cosmetic. EQUS.SUMMARY is the consolidated tape (AAPL
// 2024-08-01 = 62,500,996 shares, matching Alpaca to the share); XNAS.ITCH reports
// 21,277,576 for the same bar — correct for Nasdaq, wrong for a backtest. It carries
// ohlcv-1d / definition / statistics only, from 2024-07-01.
// ---------------------------------------------------------------------------

class DatabentoFetcher(
    private val apiKey: String = Env.require("DATABENTO_API_KEY"),
    private val dataset: String = Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY",
    private val http: HttpClient = HttpClient.newHttpClient(),
) : PricedFetcher {
    override val source = "databento:$dataset"
    override val adjusted = false          // Databento serves raw venue data

    /** Price a request before making it — usage billing makes this worth doing. */
    override fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange): Double =
        get("metadata.get_cost", symbols, schema, range).trim().toDoubleOrNull() ?: 0.0

    /** The server's own record count — the guard against silent truncation. */
    override fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange): Long =
        get("metadata.get_record_count", symbols, schema, range).trim().toLongOrNull() ?: -1

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> =
        instruments.keys.chunked(MAX_SYMBOLS_PER_REQUEST)
            .flatMap { chunk -> fetchChunk(instruments, chunk, schema, range) }

    private fun fetchChunk(
        instruments: Map<String, Long>, symbols: List<String>, schema: Schema, range: DateRange,
    ): List<BarRow> {
        val expected = runCatching { recordCount(symbols, schema, range) }.getOrDefault(-1L)
        val csv = post("timeseries.get_range", symbols, schema, range, mapOf(
            "encoding" to "csv", "map_symbols" to "true",
            "pretty_px" to "true", "pretty_ts" to "true",
        ))

        val lines = csv.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size <= 1) {
            check(expected <= 0L) { "server reports $expected records but the response was empty" }
            return emptyList()
        }
        check(expected < 0L || lines.size - 1L == expected) {
            "truncated: parsed ${lines.size - 1} rows, server reports $expected. " +
                    "Pagination is undocumented, so a partial result will not be stored."
        }

        val h = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        for (c in listOf("ts_event", "symbol", "close")) require(c in h) { "no `$c` column: ${lines.first()}" }

        return lines.drop(1).mapNotNull { line ->
            val p = line.split(",")
            val id = instruments[p[h.getValue("symbol")]] ?: return@mapNotNull null
            fun px(c: String) = h[c]?.let { p[it].toDoubleOrNull() }
            BarRow(
                ts = java.time.OffsetDateTime.parse(p[h.getValue("ts_event")]).toLocalDateTime(),
                instrumentId = id,
                open = px("open"), high = px("high"), low = px("low"), close = px("close"),
                volume = h["volume"]?.let { p[it].toLongOrNull() },
            )
        }
    }

    private fun get(endpoint: String, symbols: Collection<String>, schema: Schema, range: DateRange) =
        post(endpoint, symbols, schema, range, emptyMap())

    private fun post(endpoint: String, symbols: Collection<String>, schema: Schema,
                     range: DateRange, extra: Map<String, String>): String {
        val form = (mapOf(
            "dataset" to dataset,
            "symbols" to symbols.joinToString(","),
            "schema" to schema.wire,
            "stype_in" to "raw_symbol",
            "start" to range.from.toString(),
            "end" to range.to.plusDays(1).toString(),      // end is exclusive
        ) + extra).entries.joinToString("&") { (k, v) ->
            "${URLEncoder.encode(k, UTF_8)}=${URLEncoder.encode(v, UTF_8)}"
        }
        val auth = Base64.getEncoder().encodeToString("$apiKey:".toByteArray())
        val res = http.send(
            HttpRequest.newBuilder(URI.create("https://hist.databento.com/v0/$endpoint"))
                .header("Authorization", "Basic $auth")
                .header("Content-Type", "application/x-www-form-urlencoded")
                .POST(HttpRequest.BodyPublishers.ofString(form))
                .build(),
            HttpResponse.BodyHandlers.ofString(),
        )
        check(res.statusCode() == 200) { "databento $endpoint -> ${res.statusCode()}: ${res.body().take(300)}" }
        return res.body()
    }

    private companion object {
        /** Documented ceiling in the official client. */
        const val MAX_SYMBOLS_PER_REQUEST = 2_000
    }
}
