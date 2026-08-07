import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.*
import org.jetbrains.kotlinx.dataframe.io.readArrowFeather
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
    val df = DataFrame.readArrowFeather(File("data/bars.arrow").canonicalFile)
    val n = df.rowsCount()

    val col = df["AMD"]                                     // DataColumn<*> — boxed Doubles inside
    val amd = DoubleArray(n) { col[it] as Double }           // one unboxing copy, at load

    println("column 'AMD' -> DoubleArray(${amd.size})   first=${amd[0]}  last=${amd[n - 1]}")

    // whole frame -> one column-major matrix, the shape the strategy loop wants
    val symbols = df.columnNames().filter { it != "timestamp" }
    val close = DoubleArray(n * symbols.size)
    symbols.forEachIndexed { s, name ->
        val c = df[name]
        for (t in 0 until n) close[t + s * n] = c[t] as Double
    }
    println("frame -> close[${close.size}] column-major over ${symbols}\n")

    // --- why you do it once ---------------------------------------------
    val big = 10_000_000
    val boxed: List<Double> = List(big) { it * 0.5 }
    val prim = DoubleArray(big) { it * 0.5 }

    println("summing $big doubles:")
    bench("List<Double> (boxed)") { sumBoxed(boxed) }
    bench("DoubleArray (primitive)") { sumPrim(prim) }
    bench("convert List -> DoubleArray") { boxed.toDoubleArray().let { it[0] } }
}
