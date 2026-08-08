/**
 * Cross-check every implementation against the frozen reference weights.
 *
 * `data/weights_reference.csv` was produced by the original Python implementation
 * before it was removed. It is a golden file — regenerate it only if the strategy's
 * definition deliberately changes, never to make a failing check pass.
 */

import org.jetbrains.kotlinx.multik.ndarray.data.*
import java.io.File
import kotlin.math.abs
import kotlin.system.exitProcess

private fun loadReference(path: String): Pair<List<String>, Map<String, DoubleArray>> {
    val lines = File(path).readLines().filter { it.isNotBlank() }
    val header = lines.first().split(",")
    val syms = header.drop(1)
    val byDate = lines.drop(1).associate { line ->
        val p = line.split(",")
        p[0] to DoubleArray(syms.size) { p[it + 1].toDouble() }
    }
    return syms to byDate
}

fun main() {
    val bars = loadBars()
    val (refSyms, ref) = loadReference("data/weights_reference.csv")
    check(refSyms == bars.symbols) { "symbol order differs: $refSyms vs ${bars.symbols}" }

    val nd = bars.nd()
    val strat = MACross()

    // every implementation, keyed to the bar index its first output row corresponds to
    val impls: List<Triple<String, Array<DoubleArray>, Int>> = listOf(
        runStrategy(strat, bars.colMajor(), bars.rows, bars.cols)
            .let { (w, first) -> Triple("raw per-bar loop", w, first) },
        signalsVectorised(nd, 20, 100)
            .let { Triple("multik shifted-sum", it.toRows(), 99) },
        signalsCumsum(nd, 20, 100)
            .let { Triple("multik cumsum O(n)", it.toRows(), 99) },
    )

    var failures = 0
    for ((name, weights, first) in impls) {
        var compared = 0
        var worst = 0.0
        for (r in weights.indices) {
            val expected = ref[bars.dates[first + r].take(10)] ?: continue
            compared++
            for (j in weights[r].indices) worst = maxOf(worst, abs(weights[r][j] - expected[j]))
        }
        // a comparison that compared nothing is not a pass
        check(compared > 0) { "$name: 0 bars matched the reference — date labels are misaligned" }
        val ok = worst == 0.0
        if (!ok) failures++
        println("  ${name.padEnd(22)}${compared.toString().padStart(4)} bars   max diff ${"%.2e".format(worst)}   " +
                if (ok) "IDENTICAL" else "*** DIFFERS ***")
    }

    println()
    if (failures == 0) println("all ${impls.size} implementations match the reference")
    else { println("$failures implementation(s) diverged"); exitProcess(1) }
}

private fun D2Array<Double>.toRows(): Array<DoubleArray> =
    Array(shape[0]) { r -> DoubleArray(shape[1]) { c -> this[r, c] } }
