import org.apache.arrow.memory.RootAllocator
import org.apache.arrow.vector.Float8Vector
import org.apache.arrow.vector.TimeStampMilliVector
import org.apache.arrow.vector.ipc.ArrowFileReader
import java.io.File
import java.io.FileInputStream
import java.time.Instant
import java.time.ZoneOffset

data class MACrossParams(val fast: Int = 20, val slow: Int = 100)

/** Port of strategies/ma_cross.py — equal-weight long the names whose fast MA is above slow. */
class MACross(private val p: MACrossParams = MACrossParams()) : Strategy {
    override val lookback = p.slow

    override fun signals(w: Window, out: DoubleArray) {
        var n = 0
        for (j in 0 until w.symbols) {
            var fast = 0.0
            for (k in 0 until p.fast) fast += w.back(k, j)
            var slow = 0.0
            for (k in 0 until p.slow) slow += w.back(k, j)

            val long = fast / p.fast > slow / p.slow
            out[j] = if (long) 1.0 else 0.0
            if (long) n++
        }
        if (n > 0) for (j in 0 until w.symbols) out[j] /= n
    }
}

private class Bars(val dates: List<String>, val symbols: List<String>, val close: DoubleArray) {
    val rows get() = dates.size
}

/** Read the Arrow IPC file pandas wrote. Pure Java — no Hadoop, no native deps. */
private fun readArrow(file: File): Bars {
    RootAllocator().use { allocator ->
        val codecs = org.apache.arrow.compression.CommonsCompressionFactory.INSTANCE
        ArrowFileReader(FileInputStream(file).channel, allocator, codecs).use { reader ->
            check(reader.loadNextBatch()) { "no record batch in $file" }
            val root = reader.vectorSchemaRoot
            val rows = root.rowCount

            val ts = root.getVector("timestamp") as TimeStampMilliVector
            val dates = (0 until rows).map {
                Instant.ofEpochMilli(ts.get(it)).atZone(ZoneOffset.UTC).toLocalDate().toString()
            }

            val symbols = root.schema.fields.map { it.name }.filter { it != "timestamp" }
            val close = DoubleArray(rows * symbols.size)
            symbols.forEachIndexed { s, name ->
                val v = root.getVector(name) as Float8Vector
                for (t in 0 until rows) close[t + s * rows] = v.get(t)   // column-major
            }
            return Bars(dates, symbols, close)
        }
    }
}

fun main() {
    val here = File(".").absoluteFile.parentFile
    val bars = readArrow(File(here, "data/bars.arrow").canonicalFile)

    val strat = MACross()
    println("MACross  lookback=${strat.lookback}  params=${describe<MACrossParams>()}")
    println()

    val (weights, firstRow) = runStrategy(strat, bars.close, bars.rows, bars.symbols.size)

    println("weights, last 10 bars  (${bars.symbols.joinToString("  ")})")
    for (r in maxOf(0, weights.size - 10) until weights.size) {
        val row = weights[r].joinToString("  ") { "%.6f".format(it) }
        println("${bars.dates[firstRow + r]}   $row")
    }

    File(here, "data/weights_kotlin.csv").printWriter().use { out ->
        out.println("timestamp,${bars.symbols.joinToString(",")}")
        weights.forEachIndexed { r, row ->
            out.println("${bars.dates[firstRow + r]},${row.joinToString(",") { "%.17g".format(it) }}")
        }
    }
}
