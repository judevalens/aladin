/** DataFrame -> multik, then let the library do the linear algebra. */

import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.api.linalg.dot
import org.jetbrains.kotlinx.multik.api.linalg.solve
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*
import kotlin.random.Random

fun main() {
    // ---- DataFrame -> multik ------------------------------------------------
    val bars = loadBars()
    val rows = bars.rows
    val symbols = bars.symbols

    val close = bars.nd()

    println("close: ${close.shape.toList()}  $symbols")
    println("last 3 bars:\n${close[(rows - 3) until rows]}")

    // a window is a slice — no copy of the underlying buffer
    val window = close[(rows - 100) until rows]
    println("\nwindow ${window.shape.toList()}   mean(AMD over window) = " +
            "%.2f".format(window[0 until 100, 0 until 1].sum() / 100))

    // ---- linear algebra you don't hand-write --------------------------------
    val rng = Random(7)
    val m = 500
    val xs = DoubleArray(m) { it * 0.01 }
    val y = mk.ndarray(DoubleArray(m) { 2.5 + 1.3 * xs[it] + rng.nextDouble() - 0.5 })
    val X = mk.ndarray(DoubleArray(m * 2) { i -> if (i % 2 == 0) 1.0 else xs[i / 2] }, m, 2)

    // NOTE: transpose() returns a non-contiguous VIEW, and multik's BLAS path
    // silently produces zeros for it (with a DGEMV complaint on stderr).
    // deepCopy() materialises it. Wrong answers, no exception — watch for this.
    val xt = X.transpose().deepCopy()
    val beta = mk.linalg.solve(mk.linalg.dot(xt, X), mk.linalg.dot(xt, y))
    println("\nOLS  y ~ 1 + x   (true: intercept 2.5, slope 1.3)")
    println("  intercept %.4f   slope %.4f".format(beta[0], beta[1]))

    // ---- what BLAS buys over a hand loop ------------------------------------
    val n = 600
    val a = mk.ndarray(DoubleArray(n * n) { rng.nextDouble() }, n, n)
    val b = mk.ndarray(DoubleArray(n * n) { rng.nextDouble() }, n, n)
    val ra = Array(n) { i -> DoubleArray(n) { j -> a[i, j] } }
    val rb = Array(n) { i -> DoubleArray(n) { j -> b[i, j] } }

    fun naive(): Array<DoubleArray> {
        val c = Array(n) { DoubleArray(n) }
        for (i in 0 until n) for (p in 0 until n) {
            val aip = ra[i][p]; val br = rb[p]
            for (j in 0 until n) c[i][j] += aip * br[j]
        }
        return c
    }
    repeat(3) { naive(); mk.linalg.dot(a, b) }
    fun bench(label: String, f: () -> Any) {
        var best = Double.MAX_VALUE
        repeat(5) { val t = System.nanoTime(); f(); best = minOf(best, (System.nanoTime() - t) / 1e6) }
        println("  ${label.padEnd(28)}${"%8.1f".format(best)} ms")
    }
    println("\n${n}x$n matrix multiply:")
    bench("hand-written triple loop") { naive() }
    bench("mk.linalg.dot") { mk.linalg.dot(a, b) }
}
