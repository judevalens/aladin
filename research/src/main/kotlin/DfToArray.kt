import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.*
import java.io.File

private fun sumBoxed(xs: List<Double>): Double { var s = 0.0; for (x in xs) s += x; return s }
private fun sumPrim(xs: DoubleArray): Double { var s = 0.0; for (x in xs) s += x; return s }

private inline fun bench(label: String, reps: Int = 7, f: () -> Double) {
    f()                                                    // warm up: let the JIT compile it
    var best = Double.MAX_VALUE
    repeat(reps) {
        val t0 = System.nanoTime(); f()
        best = minOf(best, (System.nanoTime() - t0) / 1e6)
    }
    println("  ${label.padEnd(34)}${"%8.2f".format(best)} ms")
}

fun main() {
    // --- the conversion, on the real file -------------------------------
    val df = openDb().use { it.frame("SELECT close FROM bars ORDER BY ts, symbol LIMIT 2000") }
    val n = df.rowsCount()

    val col = df["close"]                                     // DataColumn<*> — boxed Doubles inside
    val amd = DoubleArray(n) { col[it] as Double }           // one unboxing copy, at load

    println("column 'close' -> DoubleArray(${amd.size})   first=${amd[0]}  last=${amd[n - 1]}")

    // for the engine, skip the DataFrame entirely — BarStore hands back both layouts
    val bars = loadBars()
    println("BarStore -> ${bars.rows} bars x ${bars.cols} symbols, " +
            "rowMajor[${bars.rowMajor.size}] and colMajor[${bars.colMajor().size}]\n")

    // --- why you do it once ---------------------------------------------
    val big = 10_000_000
    val boxed: List<Double> = List(big) { it * 0.5 }
    val prim = DoubleArray(big) { it * 0.5 }

    println("summing $big doubles:")
    bench("List<Double> (boxed)") { sumBoxed(boxed) }
    bench("DoubleArray (primitive)") { sumPrim(prim) }
    bench("convert List -> DoubleArray") { boxed.toDoubleArray().let { it[0] } }
}
