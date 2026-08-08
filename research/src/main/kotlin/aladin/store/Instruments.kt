package aladin.store

import aladin.Instrument
import aladin.InstrumentType
import aladin.normalizeSymbols
import aladin.vendor.SymbologySource
import java.sql.Connection
import java.sql.Date
import java.time.LocalDate

/**
 * Instrument identity.
 *
 * Tickers get recycled — a delisted symbol can be reassigned to an unrelated company —
 * so a symbol is a time-scoped attribute and every store here is keyed on
 * `instrument_id`. Resolution is therefore always **as-of a date**, never as-of now.
 * This is the same subtlety that produces survivorship bias, and it is very expensive
 * to retrofit.
 *
 * The registry fills on demand: a symbol enters the first time something asks for it,
 * exactly as bars enter the store on a coverage miss. Nothing is bulk-loaded.
 */
fun Connection.createInstrumentsTable() = createStatement().use {
    it.execute(
        """
        CREATE TABLE IF NOT EXISTS instruments (
          instrument_id BIGINT  NOT NULL,
          symbol        VARCHAR NOT NULL,
          valid_from    DATE    NOT NULL,
          valid_to      DATE,                    -- NULL = still current
          type          VARCHAR NOT NULL,
          PRIMARY KEY (instrument_id, valid_from)
        )
        """
    )
}

fun Connection.registerInstrument(i: Instrument): Int =
    prepareStatement(
        """
        INSERT INTO instruments (instrument_id, symbol, valid_from, valid_to, type)
        VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING
        """
    ).use { ps ->
        ps.setLong(1, i.id)
        ps.setString(2, i.symbol)
        ps.setDate(3, Date.valueOf(i.validFrom))
        ps.setDate(4, i.validTo?.let(Date::valueOf))
        ps.setString(5, i.type.name)
        ps.executeUpdate()
    }

/**
 * Which instrument did this ticker denote on [asOf]?
 *
 * Null is a real answer, not a failure: pre-IPO and post-delisting are exactly the
 * cases survivorship bias hides. Two instruments claiming one ticker on one date is an
 * identity failure and fails loudly — picking one silently attaches bars to the wrong
 * instrument, and nothing downstream could detect it.
 */
fun Connection.resolveInstrument(symbol: String, asOf: LocalDate): Long? =
    prepareStatement(
        """
        SELECT DISTINCT instrument_id FROM instruments
        WHERE symbol = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)
        """
    ).use { ps ->
        ps.setString(1, symbol)
        ps.setDate(2, Date.valueOf(asOf))
        ps.setDate(3, Date.valueOf(asOf))
        ps.executeQuery().use { rs ->
            val ids = buildList { while (rs.next()) add(rs.getLong(1)) }
            check(ids.size <= 1) { "`$symbol` resolves to ${ids.size} instruments as-of $asOf: $ids" }
            ids.firstOrNull()
        }
    }

/** The ticker an instrument carried on [asOf] — the reverse lookup, for display. */
fun Connection.symbolFor(instrumentId: Long, asOf: LocalDate): String? =
    prepareStatement(
        """
        SELECT symbol FROM instruments
        WHERE instrument_id = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to >= ?)
        ORDER BY valid_from DESC LIMIT 1
        """
    ).use { ps ->
        ps.setLong(1, instrumentId)
        ps.setDate(2, Date.valueOf(asOf))
        ps.setDate(3, Date.valueOf(asOf))
        ps.executeQuery().use { if (it.next()) it.getString(1) else null }
    }

// ---------------------------------------------------------------------------
// Read-through identity.
//
// A resolve miss is ambiguous: either we have never asked the vendor about this symbol,
// or we have and it genuinely did not exist then. Those must not cost the same. Asking
// is therefore recorded once per SYMBOL — one call returns the ticker's whole validity
// history, so every later as-of question, including ones that correctly return nothing,
// is answered locally.
// ---------------------------------------------------------------------------

fun Connection.createSymbologyTable() = createStatement().use {
    it.execute(
        """
        CREATE TABLE IF NOT EXISTS symbology_checked (
          source     VARCHAR NOT NULL,
          symbol     VARCHAR NOT NULL,
          checked_at TIMESTAMP NOT NULL DEFAULT now(),
          PRIMARY KEY (source, symbol)
        )
        """
    )
}

private fun Connection.alreadyAsked(source: String, symbol: String): Boolean =
    prepareStatement("SELECT EXISTS (SELECT 1 FROM symbology_checked WHERE source = ? AND symbol = ?)")
        .use { ps ->
            ps.setString(1, source)
            ps.setString(2, symbol)
            ps.executeQuery().use { it.next(); it.getBoolean(1) }
        }

/**
 * Resolve as-of, asking [symbology] at most once per symbol ever.
 *
 * Null still means "did not exist then" — now a recorded fact rather than an unasked
 * question.
 */
fun Connection.resolveOrFetch(symbol: String, asOf: LocalDate, symbology: SymbologySource?): Long? {
    resolveInstrument(symbol, asOf)?.let { return it }
    if (symbology == null || alreadyAsked(symbology.source, symbol)) return null

    symbology.history(symbol).forEach(::registerInstrument)
    prepareStatement("INSERT INTO symbology_checked (source, symbol) VALUES (?,?) ON CONFLICT DO NOTHING")
        .use { ps -> ps.setString(1, symbology.source); ps.setString(2, symbol); ps.executeUpdate() }
    return resolveInstrument(symbol, asOf)
}

/**
 * Resolve a universe as-of a date.
 *
 * Returns the resolved ids **and** the symbols that could not be resolved — silently
 * dropping the latter is how a universe quietly stops being as-of.
 */
fun Connection.resolveUniverse(
    symbols: List<String>,
    asOf: LocalDate,
    symbology: SymbologySource? = null,
): Pair<Map<String, Long>, List<String>> {
    val hits = LinkedHashMap<String, Long>()
    val misses = mutableListOf<String>()
    for (s in normalizeSymbols(symbols)) {
        resolveOrFetch(s, asOf, symbology)?.let { hits[s] = it } ?: misses.add(s)
    }
    return hits to misses
}

/** Convenience for tests and bootstrapping a known instrument. */
fun Connection.registerInstrument(
    id: Long,
    symbol: String,
    validFrom: LocalDate,
    validTo: LocalDate? = null,
    type: InstrumentType = InstrumentType.EQUITY,
): Int = registerInstrument(Instrument(id, symbol, validFrom, validTo, type))
