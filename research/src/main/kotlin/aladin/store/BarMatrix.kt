package aladin.store

import org.jetbrains.kotlinx.dataframe.AnyCol
import org.jetbrains.kotlinx.dataframe.AnyFrame
import org.jetbrains.kotlinx.dataframe.ColumnSelector
import org.jetbrains.kotlinx.dataframe.ColumnsSelector
import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.select
import org.jetbrains.kotlinx.multik.api.mk
import org.jetbrains.kotlinx.multik.api.ndarray
import org.jetbrains.kotlinx.multik.ndarray.data.D1Array
import org.jetbrains.kotlinx.multik.ndarray.data.D2Array

/**
 * A (time x symbol) price matrix.
 *
 * Flat storage plus shape, not nested arrays: `Array<DoubleArray>` on the JVM is an
 * array of references to separately allocated rows, costing an indirection per access
 * and scattering rows across memory. One contiguous block is also what multik and BLAS
 * want, so [nd] wraps it with no copy.
 *
 * Column labels follow **instrument_id order**, matching the grid query that produced
 * them. Ordering them alphabetically instead puts each symbol's prices under a different
 * symbol's name — silent, and total.
 */
class BarMatrix(
    val dates: List<String>,
    val symbols: List<String>,
    /** row-major (time x symbol) — the layout multik expects */
    val rowMajor: DoubleArray,
    /** Bars the store had no row for: halts, pre-listing, delisting. Surfaced on purpose. */
    val holes: Int = 0,
) {
    init {
        require(rowMajor.size == dates.size * symbols.size) {
            "ragged matrix: ${rowMajor.size} values for ${dates.size} bars x ${symbols.size} symbols"
        }
    }

    val rows: Int get() = dates.size
    val cols: Int get() = symbols.size
    val isEmpty: Boolean get() = rows == 0 || cols == 0

    operator fun get(row: Int, col: Int): Double = rowMajor[row * cols + col]

    /** Column index for a symbol, or -1. Labels are not sorted — do not binary-search them. */
    fun columnOf(symbol: String): Int = symbols.indexOf(symbol)

    /** Column-major — each symbol's series contiguous, which a per-bar loop wants. */
    fun colMajor(): DoubleArray = DoubleArray(rows * cols) { i -> rowMajor[(i % rows) * cols + i / rows] }

    /** Zero-copy 2-D view: multik wraps the same buffer rather than duplicating it. */
    fun nd(): D2Array<Double> = mk.ndarray(rowMajor, rows, cols)

    override fun toString() = "BarMatrix($rows x $cols, holes=$holes, $symbols)"
}

// ---------------------------------------------------------------------------
// DataFrame -> multik.
//
// No artifact ships this; the canonical implementation is example code in the dataframe
// repo. The selector API follows it, but the fill is deliberately different: theirs goes
// through toList(), boxing every value into a java.lang.Double at ~32 bytes against 8,
// plus an object per value for the GC. At bar-store scale that is ~16 GB against ~4 GB,
// so these write straight into a flat primitive array.
// ---------------------------------------------------------------------------

private fun List<AnyCol>.fillD2(rows: Int): D2Array<Double> {
    require(isNotEmpty()) { "no columns selected" }
    val flat = DoubleArray(rows * size)
    forEachIndexed { c, col ->
        for (r in 0 until rows) {
            flat[r * size + c] = (col[r] as? Number)?.toDouble()
                ?: error("column '${col.name()}' holds a non-numeric value at row $r")
        }
    }
    return mk.ndarray(flat, rows, size)
}

/** Selected columns as a matrix: `df.convertToMultik { colsOf<Double>() }`. */
fun <T> DataFrame<T>.convertToMultik(selector: ColumnsSelector<T, Number?>): D2Array<Double> =
    select(selector).columns().fillD2(rowsCount())

/** Every numeric column in frame order — timestamps and symbols are skipped. */
fun AnyFrame.convertToMultik(): D2Array<Double> =
    columns().filter { it.values().firstOrNull { v -> v != null } is Number }.fillD2(rowsCount())

/** One column as a 1-D array. */
fun <T> DataFrame<T>.convertToMultikD1(selector: ColumnSelector<T, Number?>): D1Array<Double> {
    val col = select(selector).columns().single()
    return mk.ndarray(DoubleArray(rowsCount()) { (col[it] as Number).toDouble() })
}
