import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

// strategy-facing: read-only interface
fun peek(w: MultiArray<Double, D2>): Double = w[0, 0]

// what a careless strategy would do if handed a mutable type
fun poison(w: MutableMultiArray<Double, D2>) { w[0, 0] = -999.0 }

fun main() {
    val bars = mk.ndarray(DoubleArray(12) { it.toDouble() }, 4, 3)
    println("bars[1,0] before = ${bars[1, 0]}")

    val window: D2Array<Double> = bars[1 until 4] as D2Array<Double>
    poison(window)                       // writes through the VIEW
    println("bars[1,0] after  = ${bars[1, 0]}   <- base array mutated")

    val safe: MultiArray<Double, D2> = bars[1 until 4]
    println("read through MultiArray = ${peek(safe)}")
    // safe[0, 0] = 1.0   // does not compile: MultiArray has no set
    println("MultiArray exposes set? ${safe is MutableMultiArray<*, *>}")
}
