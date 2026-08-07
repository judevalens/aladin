import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import java.io.File

fun main() {
    val bars = loadBars()
    val rows = bars.rows
    val syms = bars.symbols
    val close = bars.nd()
    val dates = bars.dates

    val w = signalsCumsum(close, 20, 100)
    File("data/weights_multik_cumsum.csv").printWriter().use { out ->
        out.println("timestamp,${syms.joinToString(",")}")
        for (r in 0 until w.shape[0])
            out.println("${dates[99 + r]}," + syms.indices.joinToString(",") { "%.17g".format(w[r, it]) })
    }
    println("wrote weights_multik_cumsum.csv (${w.shape[0]} rows)")
}
