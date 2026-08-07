/** Same workload as the Julia and Rust benchmarks: 2000x50 column-major, 56 param combos. */

private fun oneRun(bars: DoubleArray, tLen: Int, nLen: Int, fast: Int, slow: Int): Double {
    var pnl = 0.0
    for (n in 0 until nLen) {
        val off = n * tLen
        for (t in slow until tLen) {
            var f = 0.0
            for (k in 0 until fast) f += bars[off + t - k]
            var s = 0.0
            for (k in 0 until slow) s += bars[off + t - k]
            pnl += (if (f / fast > s / slow) 1.0 else 0.0) * (bars[off + t] - bars[off + t - 1])
        }
    }
    return pnl
}

fun main() {
    val tLen = 2000; val nLen = 50
    var seed = 0x2545F4914F6CDD1DuL
    val bars = DoubleArray(tLen * nLen) {
        seed = seed xor (seed shl 13); seed = seed xor (seed shr 7); seed = seed xor (seed shl 17)
        100.0 + (seed shr 11).toDouble() / (1L shl 53).toDouble()
    }
    val combos = (1..8).map { it * 5 }.flatMap { f -> (0..6).map { j -> f to 50 + j * 25 } }

    fun serial() = combos.sumOf { (f, s) -> oneRun(bars, tLen, nLen, f, s) }
    fun parallel() = combos.parallelStream()
        .mapToDouble { (f, s) -> oneRun(bars, tLen, nLen, f, s) }.sum()

    // JVM needs real warmup: interpreter -> C1 -> C2. Without this you benchmark the interpreter.
    repeat(20) { serial(); parallel() }

    fun bench(label: String, f: () -> Double) {
        var best = Double.MAX_VALUE
        repeat(7) { val t0 = System.nanoTime(); f(); best = minOf(best, (System.nanoTime() - t0) / 1e9) }
        println("  ${label.padEnd(28)}${"%6.3f".format(best)} s")
    }

    println("sweep of ${combos.size} runs over one shared ${tLen}x$nLen matrix")
    bench("kotlin serial", ::serial)
    bench("kotlin parallel", ::parallel)
    println("\n  cores: ${Runtime.getRuntime().availableProcessors()}")
}
