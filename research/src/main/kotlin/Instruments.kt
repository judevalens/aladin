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

data class Instrument(
    val id: Long,
    val symbol: String,
    val validFrom: LocalDate,
    val validTo: LocalDate?,
    val type: InstrumentType = InstrumentType.EQUITY,
)

fun Connection.createInstrumentsTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS instruments (
          instrument_id BIGINT  NOT NULL,
          symbol        VARCHAR NOT NULL,
          valid_from    DATE    NOT NULL,
          valid_to      DATE,                    -- NULL = still current
          type          VARCHAR NOT NULL,
          PRIMARY KEY (instrument_id, valid_from)
        )""")
}

fun Connection.registerInstrument(i: Instrument) =
    prepareStatement("""
        INSERT INTO instruments (instrument_id, symbol, valid_from, valid_to, type) VALUES (?,?,?,?,?)
        ON CONFLICT DO NOTHING""").use { ps ->
        ps.setLong(1, i.id); ps.setString(2, i.symbol)
        ps.setDate(3, Date.valueOf(i.validFrom))
        ps.setDate(4, i.validTo?.let { Date.valueOf(it) })
        ps.setString(5, i.type.name)
        ps.executeUpdate()
    }

/**
 * Which instrument did this ticker denote on [asOf]? Null when the symbol did not
 * exist then — which is a real answer, not an error: pre-IPO and post-delisting are
 * exactly the cases survivorship bias hides.
 */
fun Connection.resolveInstrument(symbol: String, asOf: LocalDate): Long? =
    prepareStatement("""
        SELECT DISTINCT instrument_id FROM instruments
        WHERE symbol = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)""").use { ps ->
        ps.setString(1, symbol); ps.setDate(2, Date.valueOf(asOf)); ps.setDate(3, Date.valueOf(asOf))
        ps.executeQuery().use { rs ->
            val ids = buildList { while (rs.next()) add(rs.getLong(1)) }
            // Two instruments claiming the same ticker on the same date is an identity
            // failure, not something to tie-break. Picking one silently attaches bars to
            // the wrong instrument, and nothing downstream can detect it.
            check(ids.size <= 1) { "`$symbol` resolves to ${ids.size} instruments as-of $asOf: $ids" }
            ids.firstOrNull()
        }
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

// ---------------------------------------------------------------------------
// Read-through identity.
//
// Nothing is bulk-loaded. A symbol enters the registry the first time something
// asks for it, exactly like bars enter the store on a coverage miss.
//
// A resolve miss is ambiguous — either we have never asked the vendor about this
// symbol, or we have and it genuinely did not exist on that date (pre-IPO,
// post-delisting). Those must not cost the same. So asking is recorded once per
// SYMBOL: after one call the vendor has told us every validity range it knows, and
// an as-of miss inside that knowledge is a real answer, not a cache miss.
// ---------------------------------------------------------------------------

/** Resolves a ticker's full validity history. UNVERIFIED against a live vendor. */
interface SymbologySource {
    val source: String
    /** Every interval this ticker denoted an instrument. Empty = the vendor knows nothing. */
    fun history(symbol: String): List<Instrument>
}

fun Connection.createSymbologyTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS symbology_checked (
          source     VARCHAR NOT NULL,
          symbol     VARCHAR NOT NULL,
          checked_at TIMESTAMP NOT NULL DEFAULT now(),
          PRIMARY KEY (source, symbol)
        )""")
}

private fun Connection.alreadyAsked(source: String, symbol: String): Boolean =
    prepareStatement("SELECT EXISTS (SELECT 1 FROM symbology_checked WHERE source = ? AND symbol = ?)")
        .use { ps -> ps.setString(1, source); ps.setString(2, symbol)
            ps.executeQuery().use { it.next(); it.getBoolean(1) } }

/**
 * Resolve as-of, asking the vendor at most once per symbol ever.
 * Null still means "did not exist then" — now a fact rather than an unasked question.
 */
fun Connection.resolveOrFetch(symbol: String, asOf: LocalDate, symbology: SymbologySource?): Long? {
    resolveInstrument(symbol, asOf)?.let { return it }
    if (symbology == null || alreadyAsked(symbology.source, symbol)) return null

    symbology.history(symbol).forEach { registerInstrument(it) }
    prepareStatement("INSERT INTO symbology_checked (source, symbol) VALUES (?,?) ON CONFLICT DO NOTHING")
        .use { ps -> ps.setString(1, symbology.source); ps.setString(2, symbol); ps.executeUpdate() }
    return resolveInstrument(symbol, asOf)
}

/** As [resolveUniverse], but fills the registry on demand. */
fun Connection.resolveUniverse(
    symbols: List<String>, asOf: LocalDate, symbology: SymbologySource?,
): Pair<Map<String, Long>, List<String>> {
    val hits = mutableMapOf<String, Long>()
    val misses = mutableListOf<String>()
    for (s in symbols) resolveOrFetch(s, asOf, symbology)?.let { hits[s] = it } ?: misses.add(s)
    return hits to misses
}
