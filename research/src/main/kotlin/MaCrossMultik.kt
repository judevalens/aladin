/**
 * MACross, vectorised with multik — no element loops.
 *
 * Rolling means are built by summing `w` shifted slices of the whole matrix, so the
 * only loop is over the window length (20 or 100), not over bars. Every operation
 * inside it is a whole-array add.
 */

import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.*
import org.jetbrains.kotlinx.dataframe.io.readArrowFeather
import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*
import java.io.File

/** Mean of the trailing `w` bars, aligned so row 0 corresponds to bar index `drop`. */
private fun rollingMean(close: D2Array<Double>, w: Int, drop: Int): D2Array<Double> {
    val rows = close.shape[0]
    val out = rows - drop
    var acc = mk.zeros<Double>(out, close.shape[1])
    for (k in 0 until w) {                       // over the WINDOW, not over bars
        acc = acc + close[(drop - k) until (rows - k)]
    }
    return acc / w.toDouble()
}

fun signalsVectorised(close: D2Array<Double>, fast: Int, slow: Int): D2Array<Double> {
    val drop = slow - 1                          // first bar with a full slow window
    val f = rollingMean(close, fast, drop)
    val s = rollingMean(close, slow, drop)

    val long = (f - s).map<Double, D2, Double> { if (it > 0.0) 1.0 else 0.0 }

    // names held per bar; where none are held the row is all zeros, so a divisor of
    // 1.0 keeps it zero instead of producing NaN
    val counts = mk.math.sumD2(long, 1).map<Double, D1, Double> { if (it > 0.0) it else 1.0 }

    val n = close.shape[1]
    val divisor = mk.ndarray(DoubleArray(long.shape[0] * n) { counts[it / n] }, long.shape[0], n)
    return long / divisor
}

fun main() {
    val df = DataFrame.readArrowFeather(File("data/bars.arrow").canonicalFile)
    val rows = df.rowsCount()
    val symbols = df.columnNames().filter { it != "timestamp" }
    val flat = DoubleArray(rows * symbols.size)
    symbols.forEachIndexed { s, name ->
        val c = df[name]
        for (t in 0 until rows) flat[t * symbols.size + s] = c[t] as Double
    }
    val close = mk.ndarray(flat, rows, symbols.size)
    val dates = df["timestamp"].values().map { it.toString().substring(0, 10) }

    val w = signalsVectorised(close, fast = 20, slow = 100)
    val first = 99

    println("weights, last 5 bars  (${symbols.joinToString("  ")})")
    for (r in w.shape[0] - 5 until w.shape[0])
        println("${dates[first + r]}   " + (0 until symbols.size).joinToString("  ") { "%.6f".format(w[r, it]) })

    File("data/weights_multik.csv").printWriter().use { out ->
        out.println("timestamp,${symbols.joinToString(",")}")
        for (r in 0 until w.shape[0])
            out.println("${dates[first + r]}," + (0 until symbols.size).joinToString(",") { "%.17g".format(w[r, it]) })
    }
}

/**
 * O(n) version: one cumulative sum, then rolling sum = cs[t+1] - cs[t+1-w].
 * Two slices and a subtract per window instead of `w` whole-array adds.
 */
fun signalsCumsum(close: D2Array<Double>, fast: Int, slow: Int): D2Array<Double> {
    val rows = close.shape[0]; val n = close.shape[1]; val drop = slow - 1

    // pad a zero row so cs[k] == sum of the first k bars, making t-w valid at t = slow-1
    val padded = mk.ndarray(DoubleArray((rows + 1) * n) { i ->
        if (i < n) 0.0 else close[(i / n) - 1, i % n]
    }, rows + 1, n)
    val cs = mk.math.cumSum(padded, 0)

    fun rollMean(w: Int): D2Array<Double> =
        ((cs[(drop + 1) until (rows + 1)] - cs[(drop + 1 - w) until (rows + 1 - w)]) / w.toDouble())
                as D2Array<Double>

    val long = (rollMean(fast) - rollMean(slow)).map<Double, D2, Double> { if (it > 0.0) 1.0 else 0.0 }
    val counts = mk.math.sumD2(long, 1).map<Double, D1, Double> { if (it > 0.0) it else 1.0 }
    val divisor = mk.ndarray(DoubleArray(long.shape[0] * n) { counts[it / n] }, long.shape[0], n)
    return long / divisor
}
