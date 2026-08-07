import org.jetbrains.kotlinx.dataframe.DataFrame
import org.jetbrains.kotlinx.dataframe.api.*
import org.jetbrains.kotlinx.dataframe.io.readArrowFeather
import org.jetbrains.kotlinx.multik.api.*
import org.jetbrains.kotlinx.multik.ndarray.data.*
import java.io.File

fun main() {
    val df = DataFrame.readArrowFeather(File("data/bars.arrow").canonicalFile)
    val rows = df.rowsCount(); val syms = df.columnNames().filter { it != "timestamp" }
    val flat = DoubleArray(rows * syms.size)
    syms.forEachIndexed { s, name -> val c = df[name]
        for (t in 0 until rows) flat[t * syms.size + s] = c[t] as Double }
    val close = mk.ndarray(flat, rows, syms.size)
    val dates = df["timestamp"].values().map { it.toString().substring(0, 10) }

    val w = signalsCumsum(close, 20, 100)
    File("data/weights_multik_cumsum.csv").printWriter().use { out ->
        out.println("timestamp,${syms.joinToString(",")}")
        for (r in 0 until w.shape[0])
            out.println("${dates[99 + r]}," + syms.indices.joinToString(",") { "%.17g".format(w[r, it]) })
    }
    println("wrote weights_multik_cumsum.csv (${w.shape[0]} rows)")
}
