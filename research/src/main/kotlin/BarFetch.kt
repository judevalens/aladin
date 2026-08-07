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
import java.util.Base64

data class BarRow(
    /** TIMESTAMP, not DATE — the same table holds 1-min and 5-min bars. */
    val ts: LocalDateTime,
    val instrumentId: Long,
    val open: Double?, val high: Double?, val low: Double?, val close: Double?,
    val volume: Long?,
)

interface BarFetcher {
    /** Vendor name recorded in the ledger, so coverage is per-source. */
    val source: String

    /** True when the vendor returns split/dividend-adjusted prices. */
    val adjusted: Boolean get() = false

    /**
     * Batched on purpose: vendors accept many symbols per request and bill by bytes,
     * so a universe backfill should be a handful of calls, not thousands.
     */
    fun fetch(instruments: Map<String, Long>, schema: String, range: DateRange): List<BarRow>
}

/**
 * Serialises every fetch. Enough here because DuckDB is single-writer and the engine
 * is one JVM — no distributed coordination needed. Costs nothing once fetches are
 * batched, since a backfill is then a few large calls rather than many small ones.
 */
class LockedFetcher(private val delegate: BarFetcher) : BarFetcher by delegate {
    private val lock = Any()
    override fun fetch(instruments: Map<String, Long>, schema: String, range: DateRange) =
        synchronized(lock) { delegate.fetch(instruments, schema, range) }
}

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
    schema: String,
    range: DateRange,
): Long {
    val settled = lastSettledSession()
    if (range.from.isAfter(settled)) return 0
    val want = DateRange(range.from, minOf(range.to, settled))

    val bySymbol = instruments.entries.flatMap { (sym, id) ->
        missingRanges(Slice(fetcher.source, id, schema), want).map { gap -> gap to (sym to id) }
    }
    if (bySymbol.isEmpty()) return 0

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
                    ps.setString(1, fetcher.source); ps.setLong(2, b.instrumentId); ps.setString(3, schema)
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
    return fetched
}

// ---------------------------------------------------------------------------
// Databento. UNVERIFIED — no API key configured, so this has never made a real
// request. Two things to confirm against live data before trusting it:
//   1. the dataset code for consolidated US equities
//   2. OHLCV prices arrive as int64 scaled by 1e-9, hence PRICE_SCALE below
// ---------------------------------------------------------------------------

private const val PRICE_SCALE = 1e-9

class DatabentoFetcher(
    private val apiKey: String = System.getenv("DATABENTO_API_KEY") ?: error("set DATABENTO_API_KEY"),
    private val dataset: String = "EQUS.SUMMARY",
    private val http: HttpClient = HttpClient.newHttpClient(),
) : BarFetcher {
    override val source = "databento"
    override val adjusted = false          // Databento serves raw venue data

    /** Price a request before making it — usage billing makes this worth doing. */
    fun estimateCostUsd(symbols: Collection<String>, schema: String, range: DateRange): Double =
        get("metadata.get_cost", symbols, schema, range).trim().toDoubleOrNull() ?: 0.0

    override fun fetch(instruments: Map<String, Long>, schema: String, range: DateRange): List<BarRow> {
        val csv = get("timeseries.get_range", instruments.keys, schema, range, "&encoding=csv")
        val lines = csv.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size <= 1) return emptyList()
        val head = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        fun px(p: List<String>, c: String) = head[c]?.let { p[it].toDoubleOrNull()?.times(PRICE_SCALE) }
        return lines.drop(1).mapNotNull { line ->
            val p = line.split(",")
            val sym = head["symbol"]?.let { p[it] } ?: instruments.keys.single()
            val id = instruments[sym] ?: return@mapNotNull null
            BarRow(
                ts = LocalDateTime.parse(p[head["ts_event"]!!].substring(0, 19).replace(' ', 'T')),
                instrumentId = id,
                open = px(p, "open"), high = px(p, "high"), low = px(p, "low"), close = px(p, "close"),
                volume = head["volume"]?.let { p[it].toLongOrNull() },
            )
        }
    }

    private fun get(endpoint: String, symbols: Collection<String>, schema: String,
                    range: DateRange, extra: String = ""): String {
        val url = "https://hist.databento.com/v0/$endpoint" +
                "?dataset=$dataset&symbols=${symbols.joinToString(",")}&schema=$schema" +
                "&start=${range.from}&end=${range.to.plusDays(1)}$extra"   // end is exclusive
        val auth = Base64.getEncoder().encodeToString("$apiKey:".toByteArray())
        val res = http.send(
            HttpRequest.newBuilder(URI.create(url)).header("Authorization", "Basic $auth").GET().build(),
            HttpResponse.BodyHandlers.ofString(),
        )
        check(res.statusCode() == 200) { "databento $endpoint -> ${res.statusCode()}: ${res.body().take(300)}" }
        return res.body()
    }
}
