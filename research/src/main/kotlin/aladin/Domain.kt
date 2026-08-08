package aladin

import java.time.LocalDate
import java.time.LocalDateTime
import java.time.temporal.ChronoUnit

/** An inclusive span of calendar days. */
data class DateRange(val from: LocalDate, val to: LocalDate) {
    init { require(!from.isAfter(to)) { "range runs backwards: $from..$to" } }
    val days: Long get() = ChronoUnit.DAYS.between(from, to) + 1
    override fun toString() = "$from..$to"
}

/**
 * Bar resolution. [barsPerSession] is regular US equity hours and is what usage-based
 * billing tracks — the jump from daily is very unevenly distributed, so the number is
 * worth having to hand rather than in a doc.
 */
enum class Schema(val wire: String, val barsPerSession: Int) {
    OHLCV_1D("ohlcv-1d", 1),
    OHLCV_1H("ohlcv-1h", 7),          // a 6.5h session leaves a partial final bar
    OHLCV_5M("ohlcv-5m", 78),
    OHLCV_1M("ohlcv-1m", 390),
    OHLCV_1S("ohlcv-1s", 23_400);

    val intraday: Boolean get() = this != OHLCV_1D

    /** Cost multiple against daily: 6.5x hourly, 390x minute, 23,400x second. */
    val vsDaily: Double get() = barsPerSession.toDouble() / OHLCV_1D.barsPerSession

    /** Approximate row count for a universe over a span. Extended hours is roughly 2.5x. */
    fun rowsFor(instruments: Int, sessions: Int): Long =
        instruments.toLong() * sessions * barsPerSession

    override fun toString() = wire

    companion object {
        fun of(wire: String): Schema = entries.firstOrNull { it.wire == wire }
            ?: error("unknown schema '$wire' (have ${entries.map(Schema::wire)})")
    }
}

/**
 * Coarse on purpose: this exists to stop a strategy treating an index level as
 * something tradeable, not to model the whole taxonomy.
 */
enum class InstrumentType {
    EQUITY, ETF, ADR,
    /** Not directly tradeable — a level, not a security. */
    INDEX,
    FUTURE, OPTION, CRYPTO,
    /** The vendor gave us nothing usable. Real, and better seen than guessed. */
    UNKNOWN;

    val tradeable: Boolean get() = this != INDEX && this != UNKNOWN
}

/**
 * An instrument over one validity window.
 *
 * Tickers get recycled, so [symbol] is a time-scoped attribute rather than identity —
 * the same subtlety that produces survivorship bias, and very expensive to retrofit.
 * A null [validTo] means "still current", not "forever".
 */
data class Instrument(
    val id: Long,
    val symbol: String,
    val validFrom: LocalDate,
    val validTo: LocalDate?,
    val type: InstrumentType = InstrumentType.EQUITY,
) {
    init { require(symbol.isNotBlank()) { "instrument $id has a blank symbol" } }
    fun coversAsOf(date: LocalDate): Boolean =
        !date.isBefore(validFrom) && (validTo == null || !date.isAfter(validTo))
}

/** One bar. Nullable prices because a halt is absence, not zero. */
data class BarRow(
    /** TIMESTAMP, not DATE — the same table holds intraday. */
    val ts: LocalDateTime,
    val instrumentId: Long,
    val open: Double?, val high: Double?, val low: Double?, val close: Double?,
    val volume: Long?,
)

/**
 * What a coverage record is about.
 *
 * [source] is the data SCOPE, not the vendor — "databento:EQUS.SUMMARY", never
 * "databento". Datasets from one vendor answer different questions: on 2024-08-01
 * AAPL is 218.36 on 62,500,996 shares consolidated, and 219.65 on 21,277,576 from
 * XNAS.ITCH. Both correct, neither a substitute, and nothing in the numbers says which.
 */
data class Slice(val source: String, val instrumentId: Long, val schema: Schema) {
    init { require(source.isNotBlank()) { "slice needs a source scope" } }
}

/** Symbols cleaned and de-duplicated, with the obvious mistakes rejected early. */
internal fun normalizeSymbols(symbols: List<String>): List<String> {
    require(symbols.isNotEmpty()) { "no symbols requested" }
    val clean = symbols.map(String::trim).filter(String::isNotEmpty).distinct()
    require(clean.isNotEmpty()) { "all requested symbols were blank" }
    return clean
}
