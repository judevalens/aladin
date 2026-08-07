import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.*
import org.jetbrains.kotlinx.dataframe.io.readArrowFeather
import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import java.io.File

fun main() {
    val df = DataFrame.readArrowFeather(File("data/bench_bars.arrow").canonicalFile)
    val rows = df.rowsCount()
    val syms = df.columnNames().filter { it != "timestamp" }
    val n = syms.size
    println("${rows} bars x $n symbols")

    // column-major for the raw loop, row-major for multik
    val colMajor = DoubleArray(rows * n)
    val rowMajor = DoubleArray(rows * n)
    syms.forEachIndexed { s, name ->
        val c = df[name]
        for (t in 0 until rows) {
            val v = c[t] as Double
            colMajor[t + s * rows] = v
            rowMajor[t * n + s] = v
        }
    }
    val nd = mk.ndarray(rowMajor, rows, n)
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
