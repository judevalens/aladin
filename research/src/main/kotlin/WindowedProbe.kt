import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

fun main() {
    val a = mk.ndarray(mk[1.0, 2.0, 3.0, 4.0, 5.0])
    val w = a.windowed(3, 1)
    println("1-D windowed(3,1) -> ${w.shape.toList()}")
    println(w)
    println("  rolling mean = ${mk.math.sumD2(w, 1) / 3.0}")

    // is it a view over the source, or a copy?
    val src = mk.ndarray(mk[1.0, 2.0, 3.0, 4.0, 5.0])
    val v = src.windowed(3, 1)
    (src as MutableMultiArray<Double, D1>)[0] = -99.0
    println("\nafter mutating source[0]: windowed[0,0] = ${v[0, 0]}  -> ${if (v[0,0] == -99.0) "VIEW" else "COPY"}")

    // what does it do to a 2-D (time x symbol) matrix?
    val m = mk.ndarray(DoubleArray(12) { it.toDouble() }, 6, 2)
    println("\n2-D input ${m.shape.toList()}:")
    println(m)
    val mw = m.windowed(3, 1)
    println("  windowed(3,1) -> ${mw.shape.toList()}")
    println(mw)
}
