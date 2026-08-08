/**
 * Typed vendor schemas and instrument kinds.
 *
 * Both are persisted by their [Schema.wire] / enum name so the stored value stays
 * readable in SQL and matches what the vendor expects on the wire.
 */

/**
 * Bar resolution. [barsPerSession] is regular US equity hours (6.5h), and is what
 * usage-based billing actually tracks — the jump from daily is very unevenly
 * distributed, so it's worth having the number to hand rather than in a doc.
 */
enum class Schema(val wire: String, val barsPerSession: Int) {
    OHLCV_1D("ohlcv-1d", 1),
    OHLCV_1H("ohlcv-1h", 7),          // 6.5h leaves a partial final bar
    OHLCV_5M("ohlcv-5m", 78),
    OHLCV_1M("ohlcv-1m", 390),
    OHLCV_1S("ohlcv-1s", 23_400);

    val intraday get() = this != OHLCV_1D

    /** Cost multiple against daily: 6.5x for hourly, 390x for minute, 23,400x for second. */
    val vsDaily get() = barsPerSession.toDouble() / OHLCV_1D.barsPerSession

    /** Rough row count for a universe over a span — extended hours roughly 2.5x this. */
    fun rowsFor(instruments: Int, sessions: Int): Long =
        instruments.toLong() * sessions * barsPerSession

    override fun toString() = wire

    companion object {
        fun of(wire: String): Schema = entries.firstOrNull { it.wire == wire }
            ?: error("unknown schema '$wire' (have ${entries.map { it.wire }})")
    }
}

/**
 * What an instrument is. Kept deliberately coarse — this exists to stop a strategy
 * treating an index level as something tradeable, not to model the whole taxonomy.
 */
enum class InstrumentType {
    EQUITY,
    ETF,
    ADR,

    /** Not directly tradeable — a level, not a security. */
    INDEX,

    FUTURE,
    OPTION,
    CRYPTO,

    /** Vendor gave us nothing usable. Real, and worth seeing rather than guessing. */
    UNKNOWN;

    val tradeable get() = this != INDEX && this != UNKNOWN
}
