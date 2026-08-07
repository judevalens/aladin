import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

fun main() {
    val rows = 5000; val n = 100; val out = rows - 99; val len = out * n
    val src = DoubleArray(rows * n) { 100.0 + it % 37 }
    val close = mk.ndarray(src, rows, n)

    // (a) what I wrote: multik, fresh array every step
    fun multikAlloc(w: Int): Any {
        var acc = mk.zeros<Double>(out, n)
        for (k in 0 until w) acc = acc + close[(99 - k) until (rows - k)]
        return acc
    }
    // (b) raw arrays, but still allocating a fresh result every step
    fun rawAlloc(w: Int): DoubleArray {
        var acc = DoubleArray(len)
        for (k in 0 until w) {
            val off = (99 - k) * n; val next = DoubleArray(len)
            for (i in 0 until len) next[i] = acc[i] + src[off + i]
            acc = next
        }
        return acc
    }
    // (c) raw arrays, accumulator reused
    fun rawInplace(w: Int): DoubleArray {
        val acc = DoubleArray(len)
        for (k in 0 until w) { val off = (99 - k) * n
            for (i in 0 until len) acc[i] += src[off + i] }
        return acc
    }

    // (d) multik, but with CONSISTENT operands (deepCopy hoisted out of the timing)
    val flatSlices = (0 until 100).map { k -> close[(99 - k) until (rows - k)].deepCopy() }
    fun multikConsistent(w: Int): Any {
        var acc = mk.zeros<Double>(out, n)
        for (k in 0 until w) acc = acc + flatSlices[k]
        return acc
    }

    fun bench(label: String, f: () -> Any) {
        repeat(10) { f() }
        var best = Double.MAX_VALUE
        repeat(7) { val t = System.nanoTime(); f(); best = minOf(best, (System.nanoTime() - t) / 1e6) }
        println("  ${label.padEnd(38)}${"%8.1f".format(best)} ms")
    }

    println("identical arithmetic — 58.8M adds, fast=20 + slow=100 over ${rows}x$n:")
    bench("(a) multik, new array per step") { multikAlloc(20); multikAlloc(100) }
    bench("(b) raw arrays, new array per step") { rawAlloc(20); rawAlloc(100) }
    bench("(c) raw arrays, reused accumulator") { rawInplace(20); rawInplace(100) }
    bench("(d) multik, consistent operands") { multikConsistent(20); multikConsistent(100) }
    println("\n  per-bar loop (register accumulator) was ~19.6 ms for the same result")
}
