import java.time.LocalDate
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread

private class SlowFetcher : BarFetcher {
    override val source = "fake"
    val calls = AtomicInteger()
    override fun fetch(symbol: String, schema: String, range: DateRange): List<BarRow> {
        calls.incrementAndGet()
        Thread.sleep(200)                                   // a vendor call takes real time
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .map { BarRow(it, symbol, 1.0, 1.0, 1.0, 1.0, 1) }.toList()
    }
}

private fun reset() = openDb().use { c ->
    c.createStatement().use { it.execute("DROP TABLE IF EXISTS coverage"); it.execute("DROP TABLE IF EXISTS ohlcv") }
    c.createCoverageTable(); c.createOhlcvTable()
}

private fun stored() = openDb().use { c ->
    c.createStatement().use { st -> st.executeQuery("SELECT count(*) FROM ohlcv").use { it.next(); it.getLong(1) } }
}

fun main() {
    val range = DateRange(LocalDate.parse("2024-01-01"), LocalDate.parse("2024-01-31"))

    println("4 threads, same symbol, same range — 31 bars expected, 1 vendor call expected\n")

    reset()
    val f1 = SlowFetcher()
    (1..4).map { thread { openDb().use { it.ensureBars(f1, "NVDA", "ohlcv-1d", range) } } }.forEach { it.join() }
    println("  unguarded:  vendor calls=${f1.calls.get()}   rows stored=${stored()}")

    // fix: serialise per slice. DuckDB is single-writer and the engine is one JVM,
    // so an in-process lock is sufficient — no distributed coordination needed.
    val locks = java.util.concurrent.ConcurrentHashMap<Slice, Any>()
    reset()
    val f2 = SlowFetcher()
    (1..4).map {
        thread {
            val key = locks.computeIfAbsent(Slice(f2.source, "NVDA", "ohlcv-1d")) { Any() }
            synchronized(key) { openDb().use { it.ensureBars(f2, "NVDA", "ohlcv-1d", range) } }
        }
    }.forEach { it.join() }
    println("  slice-locked: vendor calls=${f2.calls.get()}   rows stored=${stored()}")
}
