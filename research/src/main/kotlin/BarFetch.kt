/**
 * Read-through bar store: ask for a range, get it — fetching only what's missing.
 *
 * The whole point of the coverage ledger is that a historical range is paid for once.
 * So the store, not the caller, decides whether a fetch happens.
 */

import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.sql.Connection
import java.sql.Timestamp
import java.time.LocalDate
import java.util.Base64

data class BarRow(
    val ts: LocalDate,
    val symbol: String,
    val open: Double?, val high: Double?, val low: Double?, val close: Double?,
    val volume: Long?,
)

interface BarFetcher {
    /** Vendor name recorded in the ledger, so coverage is per-source. */
    val source: String
    fun fetch(symbol: String, schema: String, range: DateRange): List<BarRow>
}

fun Connection.createOhlcvTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS ohlcv (
          source  VARCHAR NOT NULL,
          symbol  VARCHAR NOT NULL,
          schema  VARCHAR NOT NULL,
          ts      DATE    NOT NULL,
          open DOUBLE, high DOUBLE, low DOUBLE, close DOUBLE,   -- nullable: a halt is not 0.0
          volume BIGINT
        )""")
}

/**
 * The last session that is definitely complete. Today's bar is partial until the
 * close, so caching it would freeze an incomplete day forever.
 */
fun lastSettledSession(today: LocalDate = LocalDate.now()): LocalDate = today.minusDays(1)

/**
 * Ensure [range] is held for [symbol], fetching only the gaps.
 * Returns how many bars were newly fetched — 0 means it was already a full hit.
 */
fun Connection.ensureBars(
    fetcher: BarFetcher,
    symbol: String,
    schema: String,
    range: DateRange,
): Long {
    val slice = Slice(fetcher.source, symbol, schema)
    val settled = lastSettledSession()
    if (range.from.isAfter(settled)) return 0            // nothing settled to fetch yet
    val want = DateRange(range.from, minOf(range.to, settled))

    var fetched = 0L
    for (gap in missingRanges(slice, want)) {
        val rows = fetcher.fetch(symbol, schema, gap)

        // bars and coverage must land together — a crash between them would leave the
        // ledger claiming data the store doesn't have, which is the one unrecoverable
        // failure here (you'd never re-fetch it).
        val restore = autoCommit
        autoCommit = false
        try {
            prepareStatement("INSERT INTO ohlcv VALUES (?,?,?,?,?,?,?,?,?)").use { ps ->
                for (b in rows) {
                    ps.setString(1, fetcher.source); ps.setString(2, b.symbol); ps.setString(3, schema)
                    ps.setDate(4, java.sql.Date.valueOf(b.ts))
                    ps.setObject(5, b.open); ps.setObject(6, b.high)
                    ps.setObject(7, b.low); ps.setObject(8, b.close); ps.setObject(9, b.volume)
                    ps.addBatch()
                }
                ps.executeBatch()
            }
            recordCoverage(slice, gap, rows.size.toLong())
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
    private val apiKey: String = System.getenv("DATABENTO_API_KEY")
        ?: error("set DATABENTO_API_KEY"),
    private val dataset: String = "EQUS.SUMMARY",
    private val http: HttpClient = HttpClient.newHttpClient(),
) : BarFetcher {
    override val source = "databento"

    /** Price a request before making it — usage billing makes this worth doing. */
    fun estimateCostUsd(symbol: String, schema: String, range: DateRange): Double =
        get("metadata.get_cost", symbol, schema, range).trim().toDoubleOrNull() ?: 0.0

    override fun fetch(symbol: String, schema: String, range: DateRange): List<BarRow> {
        val csv = get("timeseries.get_range", symbol, schema, range, "&encoding=csv")
        val lines = csv.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size <= 1) return emptyList()
        val head = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        fun px(p: List<String>, c: String) = head[c]?.let { p[it].toDoubleOrNull()?.times(PRICE_SCALE) }
        return lines.drop(1).map { line ->
            val p = line.split(",")
            BarRow(
                ts = LocalDate.parse(p[head["ts_event"]!!].substring(0, 10)),
                symbol = head["symbol"]?.let { p[it] } ?: symbol,
                open = px(p, "open"), high = px(p, "high"), low = px(p, "low"), close = px(p, "close"),
                volume = head["volume"]?.let { p[it].toLongOrNull() },
            )
        }
    }

    private fun get(endpoint: String, symbol: String, schema: String, range: DateRange, extra: String = ""): String {
        val url = "https://hist.databento.com/v0/$endpoint" +
                "?dataset=$dataset&symbols=$symbol&schema=$schema" +
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
