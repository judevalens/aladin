import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import org.jetbrains.kotlinx.multik.ndarray.operations.*

/** numpy's sliding_window_view: shape (n-w+1, w), strides (1, 1) over a contiguous buffer. */
private fun slidingWindow(src: D1Array<Double>, w: Int): D2Array<Double> {
    val n = src.shape[0]
    return NDArray(
        src.data, src.offset,
        intArrayOf(n - w + 1, w),
        intArrayOf(1, 1),                       // both axes step by one element — that's the trick
        D2, src,
    )
}

fun main() {
    val src = mk.ndarray(mk[1.0, 2.0, 3.0, 4.0, 5.0, 6.0])
    val win = slidingWindow(src, 3)
    println("source ${src.shape.toList()} -> window ${win.shape.toList()}")
    println(win)

    (src as MutableMultiArray<Double, D1>)[0] = -99.0
    println("\nafter source[0] = -99.0:  window[0,0] = ${win[0, 0]}  -> " +
            if (win[0, 0] == -99.0) "VIEW (zero copy)" else "COPY")

    println("\nrolling mean via the view: ${mk.math.sumD2(slidingWindow(mk.ndarray(mk[1.0,2.0,3.0,4.0,5.0,6.0]), 3), 1) / 3.0}")
}
