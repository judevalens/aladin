import java.sql.Connection
import java.time.LocalDate
import java.time.LocalDateTime
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread

/** Stands in for Databento: records every request so we can prove what was billed. */
private class CountingFetcher(private val intraday: Boolean = false) : BarFetcher {
    override val source = "fake"
    val calls = mutableListOf<Pair<DateRange, Set<String>>>()

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        synchronized(calls) { calls += range to instruments.keys.toSet() }
        Thread.sleep(20)
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .filter { it.dayOfWeek.value <= 5 }
            .flatMap { d ->
                val stamps = if (intraday) (0 until 3).map { d.atTime(9 + it, 30) } else listOf(d.atStartOfDay())
                stamps.asSequence().flatMap { ts -> instruments.values.asSequence().map { id -> ts to id } }
            }
            .map { (ts, id) -> BarRow(ts, id, 10.0, 11.0, 9.0, 10.5, 1000) }
            .toList()
    }
}

/** A fresh scratch store. Never the shared fixture — VerifyKt checks against that. */
private fun scratch(name: String, block: (Connection) -> Unit) = openScratch(name).use { c ->
    c.createCoverageTable(); c.createOhlcvTable(); c.createInstrumentsTable()
    // NVDA denotes two different instruments either side of mid-2024 — a recycled ticker
    c.registerInstrument(Instrument(1L, "NVDA", LocalDate.parse("2020-01-01"), LocalDate.parse("2024-06-30")))
    c.registerInstrument(Instrument(2L, "NVDA", LocalDate.parse("2024-07-01"), null))
    c.registerInstrument(Instrument(3L, "AMD", LocalDate.parse("2020-01-01"), null))
    block(c)
}

private fun Connection.rows() =
    createStatement().use { st -> st.executeQuery("SELECT count(*) FROM ohlcv").use { it.next(); it.getLong(1) } }

private fun d(s: String): LocalDate = LocalDate.parse(s)

fun main() {
    val march = DateRange(d("2024-03-01"), d("2024-03-31"))

    scratch("a") { c ->
        println("=== 1. as-of identity: same ticker, different instrument ===")
        for (day in listOf("2024-03-01", "2024-09-01"))
            println("  NVDA as-of $day -> instrument ${c.resolveInstrument("NVDA", d(day))}")
        val (hits, misses) = c.resolveUniverse(listOf("NVDA", "AMD", "SIVB"), d("2024-03-01"))
        println("  universe resolved=$hits  unresolvable=$misses")

        println("\n=== 2. batching: two symbols, one vendor call ===")
        val f = CountingFetcher()
        val u = c.resolveUniverse(listOf("NVDA", "AMD"), d("2024-03-01")).first
        c.ensureBars(f, u, Schema.OHLCV_1D, march)
        println("  vendor calls=${f.calls.size} -> ${f.calls.map { it.second }}   rows=${c.rows()}")

        println("\n=== 3. read-through: only gaps are billed ===")
        f.calls.clear()
        c.ensureBars(f, u, Schema.OHLCV_1D, DateRange(d("2024-03-01"), d("2024-06-30")))
        println("  extending Mar->Jun: calls=${f.calls.map { it.first }}")
        f.calls.clear()
        c.ensureBars(f, u, Schema.OHLCV_1D, DateRange(d("2024-03-01"), d("2024-06-30")))
        println("  same range again:   calls=${f.calls.map { it.first }}   (empty = full hit)")
    }

    scratch("b") { c ->
        println("\n=== 4. intraday: TIMESTAMP holds 3 bars per day ===")
        val u = c.resolveUniverse(listOf("AMD"), d("2024-03-01")).first
        c.ensureBars(CountingFetcher(intraday = true), u, Schema.OHLCV_1M,
            DateRange(d("2024-03-01"), d("2024-03-05")))
        c.createStatement().use { st ->
            st.executeQuery("SELECT ts FROM ohlcv ORDER BY ts LIMIT 4").use { rs ->
                while (rs.next()) println("    ${rs.getTimestamp(1)}")
            }
        }
    }

    scratch("c") { c ->
        println("\n=== 5. concurrency + PK: 4 threads, same slice ===")
        val locked = LockedFetcher(CountingFetcher())
        (1..4).map {
            thread { openScratch("c").use { it.ensureBars(locked, mapOf("AMD" to 3L), Schema.OHLCV_1D, march) } }
        }.forEach { it.join() }
        println("  rows stored=${c.rows()}   (21 weekdays expected; PK rejects duplicates)")

        println("\n=== 6. known-empty is recorded, not re-requested ===")
        val k = CountingFetcher()
        c.registerInstrument(Instrument(9L, "SIVB", d("2020-01-01"), d("2023-05-01")))
        val empty = DateRange(d("2024-01-01"), d("2024-01-31"))
        c.ensureBars(k, mapOf("SIVB" to 9L), Schema.OHLCV_1D, empty)
        k.calls.clear()
        c.ensureBars(k, mapOf("SIVB" to 9L), Schema.OHLCV_1D, empty)
        println("  second ask: vendor calls=${k.calls.size}  " +
                "covered=${c.isCovered(Slice("fake", 9L, Schema.OHLCV_1D), empty)}")
    }
}
