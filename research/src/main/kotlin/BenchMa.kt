import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*

fun main() {
    val bars = loadBars("bench_bars")
    val rows = bars.rows
    val syms = bars.symbols
    val n = syms.size
    println("${rows} bars x $n symbols")

    val colMajor = bars.colMajor()
    val nd = bars.nd()
    val strat = MACross()

    fun bench(label: String, f: () -> Any) {
        repeat(15) { f() }                                   // JIT warmup
        var best = Double.MAX_VALUE
        repeat(7) { val t = System.nanoTime(); f(); best = minOf(best, (System.nanoTime() - t) / 1e6) }
        println("  ${label.padEnd(28)}${"%8.1f".format(best)} ms")
    }

    bench("kotlin (per-bar loop)") { runStrategy(strat, colMajor, rows, n) }
    bench("kotlin (multik shifted-sum)") { signalsVectorised(nd, 20, 100) }
    bench("kotlin (multik cumsum, O(n))") { signalsCumsum(nd, 20, 100) }
}
