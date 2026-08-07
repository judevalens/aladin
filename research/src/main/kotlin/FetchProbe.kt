import java.time.LocalDate
import java.time.LocalDateTime
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.thread

/** Stands in for Databento: records every request so we can prove what was billed. */
private class CountingFetcher(private val intraday: Boolean = false) : BarFetcher {
    override val source = "fake"
    val calls = mutableListOf<Pair<DateRange, Set<String>>>()
    val callCount = AtomicInteger()

    override fun fetch(instruments: Map<String, Long>, schema: String, range: DateRange): List<BarRow> {
        synchronized(calls) { calls += range to instruments.keys.toSet() }
        callCount.incrementAndGet()
        Thread.sleep(50)
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

private fun fresh() = openDb().use { c ->
    c.createStatement().use {
        it.execute("DROP TABLE IF EXISTS coverage"); it.execute("DROP TABLE IF EXISTS ohlcv")
        it.execute("DROP TABLE IF EXISTS instruments")
    }
    c.createCoverageTable(); c.createOhlcvTable(); c.createInstrumentsTable()
    // NVDA's id changes mid-2024 — a recycled ticker, the case symbol-keying gets wrong
    c.registerInstrument(Instrument(1L, "NVDA", LocalDate.parse("2020-01-01"), LocalDate.parse("2024-06-30")))
    c.registerInstrument(Instrument(2L, "NVDA", LocalDate.parse("2024-07-01"), null))
    c.registerInstrument(Instrument(3L, "AMD", LocalDate.parse("2020-01-01"), null))
}

private fun rows() = openDb().use { c ->
    c.createStatement().use { st -> st.executeQuery("SELECT count(*) FROM ohlcv").use { it.next(); it.getLong(1) } }
}

fun main() {
    println("=== 1. as-of identity: same ticker, different instrument ===")
    fresh()
    openDb().use { c ->
        for (d in listOf("2024-03-01", "2024-09-01")) {
            println("  NVDA as-of $d -> instrument ${c.resolveInstrument("NVDA", LocalDate.parse(d))}")
        }
        val (hits, misses) = c.resolveUniverse(listOf("NVDA", "AMD", "SIVB"), LocalDate.parse("2024-03-01"))
        println("  universe resolved=$hits  unresolvable=$misses")
    }

    println("\n=== 2. batching: two symbols, one vendor call ===")
    fresh()
    val f = CountingFetcher()
    openDb().use { c ->
        val u = c.resolveUniverse(listOf("NVDA", "AMD"), LocalDate.parse("2024-03-01")).first
        c.ensureBars(f, u, "ohlcv-1d", DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-03-31")))
        println("  vendor calls=${f.calls.size}  -> ${f.calls.map { it.second }}   rows=${rows()}")
    }

    println("\n=== 3. read-through: only gaps are billed ===")
    val g = CountingFetcher()
    openDb().use { c ->
        val u = c.resolveUniverse(listOf("NVDA", "AMD"), LocalDate.parse("2024-03-01")).first
        g.calls.clear()
        c.ensureBars(g, u, "ohlcv-1d", DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-06-30")))
        println("  extending Mar->Jun: calls=${g.calls.map { it.first }}")
        g.calls.clear()
        c.ensureBars(g, u, "ohlcv-1d", DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-06-30")))
        println("  same range again:   calls=${g.calls.map { it.first }}   (empty = full hit)")
    }

    println("\n=== 4. intraday: TIMESTAMP holds 3 bars per day ===")
    fresh()
    val h = CountingFetcher(intraday = true)
    openDb().use { c ->
        val u = c.resolveUniverse(listOf("AMD"), LocalDate.parse("2024-03-01")).first
        c.ensureBars(h, u, "ohlcv-1m", DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-03-05")))
        c.createStatement().use { st ->
            st.executeQuery("SELECT ts FROM ohlcv ORDER BY ts LIMIT 4").use { rs ->
                while (rs.next()) println("    ${rs.getTimestamp(1)}")
            }
        }
    }

    println("\n=== 5. concurrency + PK: 4 threads, same slice ===")
    fresh()
    val locked = LockedFetcher(CountingFetcher())
    val range = DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-03-31"))
    (1..4).map {
        thread { openDb().use { c ->
            c.ensureBars(locked, mapOf("AMD" to 3L), "ohlcv-1d", range)
        } }
    }.forEach { it.join() }
    println("  rows stored=${rows()}   (21 weekdays expected; PK rejects any duplicate)")

    println("\n=== 6. known-empty is recorded, not re-requested ===")
    val k = CountingFetcher()
    openDb().use { c ->
        c.registerInstrument(Instrument(9L, "SIVB", LocalDate.parse("2020-01-01"), LocalDate.parse("2023-05-01")))
        val empty = DateRange(LocalDate.parse("2024-01-01"), LocalDate.parse("2024-01-31"))
        c.ensureBars(k, mapOf("SIVB" to 9L), "ohlcv-1d", empty)
        k.calls.clear()
        c.ensureBars(k, mapOf("SIVB" to 9L), "ohlcv-1d", empty)
        println("  second ask: vendor calls=${k.calls.size}  covered=${c.isCovered(Slice("fake", 9L, "ohlcv-1d"), empty)}")
    }
}
