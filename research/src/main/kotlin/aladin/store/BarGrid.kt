package aladin.store

import aladin.DateRange
import aladin.Schema
import aladin.normalizeSymbols
import aladin.vendor.BarFetcher
import aladin.vendor.SymbologySource
import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.io.readResultSet
import java.sql.Connection
import java.sql.PreparedStatement
import java.sql.Timestamp
import java.time.LocalDate

/** What a missing bar becomes. Changes what a backtest says — choose deliberately. */
enum class Holes {
    /** Honest: the strategy must cope. A lookback spanning a halt yields NaN unless it is careful. */
    NAN,

    /** Convenient, and invents a price that never traded — a strategy can "hold" a halted name. */
    FORWARD_FILL,

    /** Keeps the matrix clean; loses a bar for every instrument because one was missing. */
    DROP_DATE,
}

/**
 * A dense (dates x instruments) grid.
 *
 * The store is legitimately ragged — a halt, a mid-range IPO and a delisting all lack
 * rows — but a matrix must be rectangular. CROSS JOINing the grid and LEFT JOINing the
 * bars makes that structural rather than something the loader hopes for, and leaves the
 * holes explicit as NULL instead of silently absent.
 *
 * `field` is interpolated because SQL cannot bind an identifier, so it is checked
 * against [PRICE_FIELDS] first. Instrument ids are Longs, so they carry no injection
 * risk.
 */
private fun denseGridSql(field: String, ids: Collection<Long>): String {
    requireIdentifier(field, PRICE_FIELDS, "price field")
    val idList = ids.joinToString(",")
    return """
        WITH cal AS (
            SELECT DISTINCT ts FROM ohlcv
            WHERE source = ? AND schema = ? AND ts >= ? AND ts < ?
              AND instrument_id IN ($idList)
        ), inst AS (SELECT unnest([$idList]) AS instrument_id)
        SELECT c.ts, i.instrument_id, o.$field AS value
        FROM cal c CROSS JOIN inst i
        LEFT JOIN ohlcv o
               ON o.ts = c.ts AND o.instrument_id = i.instrument_id
              AND o.source = ? AND o.schema = ?
        ORDER BY c.ts, i.instrument_id
    """
}

/**
 * The same dense grid, but every OHLCV column plus the ticker as-of the bar.
 *
 * The matrix takes one field because it is a numeric matrix; a frame is for looking, so
 * it carries the whole bar. The symbol comes from the instruments table joined **as-of
 * the bar's own date**, so a recycled ticker reads as whatever it was called then rather
 * than whatever it is called now.
 *
 * The instruments join hangs off the grid's instrument, not the bar's, so a hole still
 * shows its symbol with NULL prices rather than dropping out of the frame.
 */
private fun denseGridAllSql(ids: Collection<Long>): String {
    val idList = ids.joinToString(",")
    return """
        WITH cal AS (
            SELECT DISTINCT ts FROM ohlcv
            WHERE source = ? AND schema = ? AND ts >= ? AND ts < ?
              AND instrument_id IN ($idList)
        ), inst AS (SELECT unnest([$idList]) AS instrument_id)
        SELECT c.ts,
               sym.symbol,
               i.instrument_id,
               o.open, o.high, o.low, o.close, o.volume,
               o.adjusted
        FROM cal c
        CROSS JOIN inst i
        LEFT JOIN ohlcv o
               ON o.ts = c.ts AND o.instrument_id = i.instrument_id
              AND o.source = ? AND o.schema = ?
        LEFT JOIN instruments sym
               ON sym.instrument_id = i.instrument_id
              AND c.ts::DATE >= sym.valid_from
              AND (sym.valid_to IS NULL OR c.ts::DATE <= sym.valid_to)
        ORDER BY c.ts, i.instrument_id
    """
}

private fun Connection.gridStatement(
    source: String,
    schema: Schema,
    field: String,
    ids: List<Long>,
    range: DateRange,
): PreparedStatement = bindGrid(denseGridSql(field, ids), source, schema, range)

private fun Connection.bindGrid(
    sql: String,
    source: String,
    schema: Schema,
    range: DateRange,
): PreparedStatement = prepareStatement(sql).apply {
    setString(1, source); setString(2, schema.wire)
    setTimestamp(3, Timestamp.valueOf(range.from.atStartOfDay()))
    setTimestamp(4, Timestamp.valueOf(range.to.plusDays(1).atStartOfDay()))
    setString(5, source); setString(6, schema.wire)
}

/**
 * Resolve as-of, optionally fetch what is missing, and return ids in the order the grid
 * query will emit them.
 *
 * The ordering is load-bearing: the query emits columns ordered by `instrument_id`, so
 * the labels must be too. Sorting labels alphabetically instead puts every symbol's
 * prices under a different symbol's name whenever the two orders disagree — which is
 * most of the time.
 */
private fun Connection.prepareUniverse(
    symbols: List<String>,
    asOf: LocalDate,
    range: DateRange,
    schema: Schema,
    fetcher: BarFetcher?,
    symbology: SymbologySource?,
): Pair<List<String>, List<Long>> {
    val (hits, misses) = resolveUniverse(normalizeSymbols(symbols), asOf, symbology)
    require(hits.isNotEmpty()) {
        "none of $misses resolve to an instrument as-of $asOf — " +
            "either the symbols are wrong or nothing has taught the registry about them"
    }
    fetcher?.let { ensureBars(it, hits, schema, range) }

    val ordered = hits.entries.sortedBy { it.value }
    return ordered.map { it.key } to ordered.map { it.value }
}

/** Bars as a [BarMatrix] — straight from the ResultSet into flat primitives, nothing boxed. */
fun Connection.loadMatrix(
    symbols: List<String>,
    range: DateRange,
    asOf: LocalDate = range.from,
    field: String = "close",
    schema: Schema = Schema.OHLCV_1D,
    source: String,
    holes: Holes = Holes.NAN,
    fetcher: BarFetcher? = null,
    symbology: SymbologySource? = null,
): BarMatrix {
    val (syms, ids) = prepareUniverse(symbols, asOf, range, schema, fetcher, symbology)

    val dates = mutableListOf<String>()
    val values = mutableListOf<Double>()
    var missing = 0
    gridStatement(source, schema, field, ids, range).use { ps ->
        ps.executeQuery().use { rs ->
            var last = ""
            while (rs.next()) {
                val ts = rs.getTimestamp(1).toLocalDateTime().toString()
                if (ts != last) { dates += ts; last = ts }
                val v = rs.getDouble(3)
                if (rs.wasNull()) { missing++; values += Double.NaN } else values += v
            }
        }
    }

    val m = BarMatrix(dates, syms, values.toDoubleArray(), missing)
    return if (missing == 0) m else when (holes) {
        Holes.NAN -> m
        Holes.FORWARD_FILL -> m.forwardFilled()
        Holes.DROP_DATE -> m.withoutIncompleteDates()
    }
}

/**
 * The whole bar as a DataFrame — every OHLCV column, the ticker as-of, and the
 * adjusted flag. Boxed and printable, holes as null. For looking, not for looping.
 */
fun Connection.loadFrame(
    symbols: List<String>,
    range: DateRange,
    asOf: LocalDate = range.from,
    schema: Schema = Schema.OHLCV_1D,
    source: String,
    fetcher: BarFetcher? = null,
    symbology: SymbologySource? = null,
): AnyFrame {
    val (_, ids) = prepareUniverse(symbols, asOf, range, schema, fetcher, symbology)
    return bindGrid(denseGridAllSql(ids), source, schema, range).use { ps ->
        ps.executeQuery().use { DataFrame.readResultSet(it, DuckDb) }
    }
}

/** Carry the last observed value forward per instrument. A leading hole stays NaN — nothing to carry. */
private fun BarMatrix.forwardFilled(): BarMatrix {
    val out = rowMajor.copyOf()
    for (c in 0 until cols) {
        var last = Double.NaN
        for (r in 0 until rows) {
            val i = r * cols + c
            if (out[i].isNaN()) out[i] = last else last = out[i]
        }
    }
    return BarMatrix(dates, symbols, out, holes)
}

/** Drop any date where an instrument is missing, so every remaining row is complete. */
private fun BarMatrix.withoutIncompleteDates(): BarMatrix {
    val keep = (0 until rows).filter { r -> (0 until cols).none { rowMajor[r * cols + it].isNaN() } }
    val out = DoubleArray(keep.size * cols)
    keep.forEachIndexed { newRow, r -> System.arraycopy(rowMajor, r * cols, out, newRow * cols, cols) }
    return BarMatrix(keep.map { dates[it] }, symbols, out, holes)
}
