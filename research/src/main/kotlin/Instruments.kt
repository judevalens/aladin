/**
 * Instrument identity.
 *
 * Tickers get recycled — a delisted symbol can be reassigned to an unrelated company.
 * So `symbol` is a time-scoped *attribute* of an instrument, not its identity, and
 * every store here is keyed on `instrument_id`. This is the same subtlety that
 * produces survivorship bias, and it is very expensive to retrofit.
 *
 * Resolution is therefore always **as-of a date**, never as-of now.
 *
 * The registry is deliberately thin. Full as-of universe resolution needs the vendor's
 * symbology (Databento resolves symbol -> instrument_id per date and retains delisted
 * instruments); this holds the mapping once something has told us about it.
 */

import java.sql.Connection
import java.sql.Date
import java.time.LocalDate

data class Instrument(val id: Long, val symbol: String, val validFrom: LocalDate, val validTo: LocalDate?)

fun Connection.createInstrumentsTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS instruments (
          instrument_id BIGINT  NOT NULL,
          symbol        VARCHAR NOT NULL,
          valid_from    DATE    NOT NULL,
          valid_to      DATE,                    -- NULL = still current
          PRIMARY KEY (instrument_id, valid_from)
        )""")
}

fun Connection.registerInstrument(i: Instrument) =
    prepareStatement("""
        INSERT INTO instruments (instrument_id, symbol, valid_from, valid_to) VALUES (?,?,?,?)
        ON CONFLICT DO NOTHING""").use { ps ->
        ps.setLong(1, i.id); ps.setString(2, i.symbol)
        ps.setDate(3, Date.valueOf(i.validFrom))
        ps.setDate(4, i.validTo?.let { Date.valueOf(it) })
        ps.executeUpdate()
    }

/**
 * Which instrument did this ticker denote on [asOf]? Null when the symbol did not
 * exist then — which is a real answer, not an error: pre-IPO and post-delisting are
 * exactly the cases survivorship bias hides.
 */
fun Connection.resolveInstrument(symbol: String, asOf: LocalDate): Long? =
    prepareStatement("""
        SELECT instrument_id FROM instruments
        WHERE symbol = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)
        ORDER BY valid_from DESC LIMIT 1""").use { ps ->
        ps.setString(1, symbol); ps.setDate(2, Date.valueOf(asOf)); ps.setDate(3, Date.valueOf(asOf))
        ps.executeQuery().use { if (it.next()) it.getLong(1) else null }
    }

/** The ticker this instrument carried on [asOf] — the reverse lookup, for display. */
fun Connection.symbolFor(instrumentId: Long, asOf: LocalDate): String? =
    prepareStatement("""
        SELECT symbol FROM instruments
        WHERE instrument_id = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)
        ORDER BY valid_from DESC LIMIT 1""").use { ps ->
        ps.setLong(1, instrumentId); ps.setDate(2, Date.valueOf(asOf)); ps.setDate(3, Date.valueOf(asOf))
        ps.executeQuery().use { if (it.next()) it.getString(1) else null }
    }

/**
 * Resolve a universe as-of a date, dropping names that did not exist then.
 * Returns the resolved ids plus the symbols that could not be resolved, because
 * silently dropping them is how a universe quietly stops being as-of.
 */
fun Connection.resolveUniverse(symbols: List<String>, asOf: LocalDate): Pair<Map<String, Long>, List<String>> {
    val hits = mutableMapOf<String, Long>()
    val misses = mutableListOf<String>()
    for (s in symbols) resolveInstrument(s, asOf)?.let { hits[s] = it } ?: misses.add(s)
    return hits to misses
}
