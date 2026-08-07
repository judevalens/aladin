/** Minimal reproduction: mk.linalg.dot on a transposed VIEW vs a materialised copy. */

import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.api.linalg.dot
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

fun main() {
    // A = 3x2   [[1,2],[3,4],[5,6]]
    val a = mk.ndarray(doubleArrayOf(1.0, 2.0, 3.0, 4.0, 5.0, 6.0), 3, 2)
    val y = mk.ndarray(doubleArrayOf(1.0, 1.0, 1.0))          // 3-vector

    val view = a.transpose()          // 2x3 VIEW — same buffer, swapped strides
    val copy = a.transpose().deepCopy() // 2x3 — freshly laid out, contiguous

    println("A =\n$a")
    println("A.transpose() prints correctly either way:")
    println("  view = $view")
    println("  copy = $copy")
    println("  same contents? ${view.toList() == copy.toList()}")
    println("  view consistent? ${view.consistent}   copy consistent? ${copy.consistent}")

    println("\nexpected  A' * A = [[35,44],[44,56]]   A' * y = [9,12]")
    println("  dot(view, a) =\n${mk.linalg.dot(view, a)}")
    println("  dot(copy, a) =\n${mk.linalg.dot(copy, a)}")
    println("  dot(view, y) = ${mk.linalg.dot(view, y)}")
    println("  dot(copy, y) = ${mk.linalg.dot(copy, y)}")
}
