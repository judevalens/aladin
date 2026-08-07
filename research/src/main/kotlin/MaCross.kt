import java.io.File

data class MACrossParams(val fast: Int = 20, val slow: Int = 100)

/** Port of strategies/ma_cross.py — equal-weight long the names whose fast MA is above slow. */
class MACross(private val p: MACrossParams = MACrossParams()) : Strategy {
    override val lookback = p.slow

    override fun signals(w: Window, out: DoubleArray) {
        var n = 0
        for (j in 0 until w.symbols) {
            var fast = 0.0
            for (k in 0 until p.fast) fast += w.back(k, j)
            var slow = 0.0
            for (k in 0 until p.slow) slow += w.back(k, j)

            val long = fast / p.fast > slow / p.slow
            out[j] = if (long) 1.0 else 0.0
            if (long) n++
        }
        if (n > 0) for (j in 0 until w.symbols) out[j] /= n
    }
}

fun main() {
    val bars = loadBars()

    val strat = MACross()
    println("MACross  lookback=${strat.lookback}  params=${describe<MACrossParams>()}")
    println()

    val (weights, firstRow) = runStrategy(strat, bars.colMajor(), bars.rows, bars.cols)

    println("weights, last 10 bars  (${bars.symbols.joinToString("  ")})")
    for (r in maxOf(0, weights.size - 10) until weights.size) {
        val row = weights[r].joinToString("  ") { "%.6f".format(it) }
        println("${bars.dates[firstRow + r]}   $row")
    }

    File("data/weights_kotlin.csv").printWriter().use { out ->
        out.println("timestamp,${bars.symbols.joinToString(",")}")
        weights.forEachIndexed { r, row ->
            out.println("${bars.dates[firstRow + r]},${row.joinToString(",") { "%.17g".format(it) }}")
        }
    }
}
