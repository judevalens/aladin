/**
 * The store's interface for strategies. No SQL leaves this file.
 *
 * A strategy that wrote its own query would have to re-derive three guarantees, and
 * would eventually get one wrong: the universe resolved **as-of** the range start (not
 * as-of now, which is survivorship bias), coverage checked so it can't silently read a
 * range that was never fetched, and a **rectangular** result even though the store is
 * legitimately ragged.
 */

import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.io.readResultSet
import java.sql.Connection
import java.sql.Timestamp
import java.time.LocalDate

/** What a missing bar becomes in the matrix. Changes what a backtest says — choose deliberately. */
enum class Holes {
    /** Honest: the strategy must cope. A lookback spanning a halt yields NaN unless it's careful. */
    NAN,

    /** Convenient, and invents a price that never traded — a strategy can "hold" a halted name. */
    FORWARD_FILL,

    /** Keeps the matrix clean; loses a bar for every instrument because one was missing. */
    DROP_DATE,
}

/**
 * Dense (dates × instruments) grid. The CROSS JOIN makes rectangularity structural, so
 * the loader never reasons about raggedness and holes arrive as explicit NULLs.
 */
private fun denseGridSql(schema: Schema, field: String, ids: Collection<Long>) = """
    WITH cal AS (
        SELECT DISTINCT ts FROM ohlcv
        WHERE source = ? AND schema = ? AND ts >= ? AND ts < ? AND instrument_id IN (${ids.joinToString(",")})
    ), inst AS (SELECT unnest([${ids.joinToString(",")}]) AS instrument_id)
    SELECT c.ts, i.instrument_id, o.$field AS value
    FROM cal c CROSS JOIN inst i
    LEFT JOIN ohlcv o
           ON o.ts = c.ts AND o.instrument_id = i.instrument_id
          AND o.source = ? AND o.schema = ?
    ORDER BY c.ts, i.instrument_id
"""

private fun Connection.gridStatement(
    source: String, schema: Schema, field: String, ids: List<Long>, range: DateRange,
) = prepareStatement(denseGridSql(schema, field, ids)).apply {
    setString(1, source); setString(2, schema.wire)
    setTimestamp(3, Timestamp.valueOf(range.from.atStartOfDay()))
    setTimestamp(4, Timestamp.valueOf(range.to.plusDays(1).atStartOfDay()))
    setString(5, source); setString(6, schema.wire)
}

/** Resolve as-of, ensure coverage, and return the instrument ids in stable symbol order. */
private fun Connection.prepareUniverse(
    symbols: List<String>, asOf: LocalDate, range: DateRange,
    schema: Schema, fetcher: BarFetcher?,
): Pair<List<String>, List<Long>> {
    val (hits, misses) = resolveUniverse(symbols.sorted(), asOf)
    require(hits.isNotEmpty()) { "no symbols resolve as-of $asOf (unresolved: $misses)" }
    fetcher?.let { ensureBars(it, hits, schema, range) }
    val ordered = hits.keys.sorted()
    return ordered to ordered.map { hits.getValue(it) }
}

/**
 * Bars as a [BarMatrix] — the engine path. Straight from the ResultSet into flat
 * primitive arrays; nothing is boxed on the way.
 */
fun Connection.loadMatrix(
    symbols: List<String>,
    range: DateRange,
    asOf: LocalDate = range.from,
    field: String = "close",
    schema: Schema = Schema.OHLCV_1D,
    source: String = "fixture",
    holes: Holes = Holes.NAN,
    fetcher: BarFetcher? = null,
): BarMatrix {
    val (syms, ids) = prepareUniverse(symbols, asOf, range, schema, fetcher)

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
    check(values.size == dates.size * ids.size) { "grid was not rectangular — check the CROSS JOIN" }

    var m = BarMatrix(dates, syms, values.toDoubleArray(), missing)
    if (missing > 0) m = when (holes) {
        Holes.NAN -> m
        Holes.FORWARD_FILL -> m.forwardFilled()
        Holes.DROP_DATE -> m.withoutIncompleteDates()
    }
    return m
}

/** The same query as a [AnyFrame] — the exploration path. Boxed, printable, holes as null. */
fun Connection.loadFrame(
    symbols: List<String>,
    range: DateRange,
    asOf: LocalDate = range.from,
    field: String = "close",
    schema: Schema = Schema.OHLCV_1D,
    source: String = "fixture",
    fetcher: BarFetcher? = null,
): AnyFrame {
    val (_, ids) = prepareUniverse(symbols, asOf, range, schema, fetcher)
    return gridStatement(source, schema, field, ids, range).use { ps ->
        ps.executeQuery().use { DataFrame.readResultSet(it, DuckDb) }
    }
}

/** Carry the last observed value forward per instrument. Leading NaNs stay NaN — nothing to carry. */
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
    keep.forEachIndexed { newR, r ->
        System.arraycopy(rowMajor, r * cols, out, newR * cols, cols)
    }
    return BarMatrix(keep.map { dates[it] }, symbols, out, holes)
}
