import java.sql.DriverManager
import java.time.LocalDate

fun main() {
    fun once(): Int {
    // no path = in-memory, torn down with the connection
    openScratch().use { c ->
        c.createInstrumentsTable(); c.createOhlcvTable(); c.createCoverageTable()
        c.registerInstrument(Instrument(1L, "TEST", LocalDate.parse("2020-01-01"), null))

        val f = object : BarFetcher {
            override val source = "fake"
            override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange) =
                generateSequence(range.from) { it.plusDays(1) }
                    .takeWhile { !it.isAfter(range.to) }
                    .map { BarRow(it.atStartOfDay(), 1L, 1.0, 1.0, 1.0, 42.0, 1) }.toList()
        }
        val range = DateRange(LocalDate.parse("2024-01-01"), LocalDate.parse("2024-01-31"))
        c.ensureBars(f, mapOf("TEST" to 1L), Schema.OHLCV_1D, range)
        val m = c.loadMatrix(listOf("TEST"), range, source = "fake")
        return m.rows
    }
    }

    println("  first call (JVM + driver load): ${"%.0f".format(time { once() })} ms")
    repeat(3) { once() }
    val marginal = (1..10).map { time { once() } }.sorted()[5]
    println("  marginal, per fresh in-memory DB: ${"%.1f".format(marginal)} ms")
}

private inline fun time(f: () -> Unit): Double {
    val t = System.nanoTime(); f(); return (System.nanoTime() - t) / 1e6
}
