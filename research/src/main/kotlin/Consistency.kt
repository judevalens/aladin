import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

fun main() {
    val a = mk.ndarray(DoubleArray(18) { it.toDouble() }, 6, 3)   // 6 bars x 3 symbols

    fun show(label: String, x: MultiArray<Double, *>) =
        println("  ${label.padEnd(38)}shape=${x.shape.toList().toString().padEnd(8)} consistent=${x.consistent}")

    println("6x3 bars matrix — which operations stay contiguous?")
    show("original", a)
    show("a[2 until 6]           leading rows", a[2 until 6])
    show("a[2 until 6, 0 until 3]  explicit", a[2 until 6, 0 until 3])
    show("a[0 until 6, 1 until 2]  one column", a[0 until 6, 1 until 2])
    show("a.transpose()", a.transpose())
    show("a.transpose().deepCopy()", a.transpose().deepCopy())
    show("a[2 until 6].deepCopy()", a[2 until 6].deepCopy())
}
