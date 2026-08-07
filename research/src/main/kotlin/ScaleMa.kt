import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*

fun main() {
    val rows = 5000; val n = 100
    val flat = DoubleArray(rows * n) { 100.0 + (it % 37) * 0.3 }
    val nd = mk.ndarray(flat, rows, n)
    val colMajor = DoubleArray(rows * n) { i -> flat[(i % rows) * n + i / rows] }

    fun bench(f: () -> Any): Double {
        repeat(8) { f() }
        var best = Double.MAX_VALUE
        repeat(5) { val t = System.nanoTime(); f(); best = minOf(best, (System.nanoTime() - t) / 1e6) }
        return best
    }

    println("how each scales with lookback (5000 bars x 100 symbols)")
    println("  ${"slow".padEnd(8)}${"per-bar loop".padStart(14)}${"multik cumsum".padStart(16)}")
    for (slow in listOf(50, 100, 250, 500)) {
        val strat = MACross(MACrossParams(fast = 20, slow = slow))
        val loop = bench { runStrategy(strat, colMajor, rows, n) }
        val cum = bench { signalsCumsum(nd, 20, slow) }
        println("  ${slow.toString().padEnd(8)}${"%11.1f ms".format(loop)}${"%13.1f ms".format(cum)}")
    }
}
