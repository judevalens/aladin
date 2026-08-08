import kotlin.math.abs

fun main() {
    val n = 200_000; val w = 250
    // realistic magnitudes: prices near 400, so cumsum reaches ~8e7
    val x = DoubleArray(n) { 400.0 + (it % 7919) * 0.001 }

    fun naive(t: Int): Double { var s = 0.0; for (k in 0 until w) s += x[t - k]; return s }

    val running = DoubleArray(n); run {
        var s = 0.0
        for (i in 0 until n) { s += x[i]; if (i >= w) s -= x[i - w]; if (i >= w - 1) running[i] = s }
    }
    val cum = DoubleArray(n + 1); for (i in 0 until n) cum[i + 1] = cum[i] + x[i]

    var maxRun = 0.0; var maxCum = 0.0
    for (t in w - 1 until n) {
        val exact = naive(t)
        maxRun = maxOf(maxRun, abs(running[t] - exact))
        maxCum = maxOf(maxCum, abs((cum[t + 1] - cum[t + 1 - w]) - exact))
    }
    println("$n bars, window $w — max deviation from a freshly-summed window:")
    println("  running sum   ${"%.3e".format(maxRun)}")
    println("  cumsum        ${"%.3e".format(maxCum)}")
    println("  cumsum reaches ${"%.3e".format(cum[n])}, individual prices ~4e2")
}
