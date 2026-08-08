import java.sql.Connection
import java.time.LocalDate
import kotlin.test.*

private fun d(s: String) = LocalDate.parse(s)
private fun r(a: String, b: String) = DateRange(d(a), d(b))

/** Each test gets its own in-memory store. ~43ms, so there is no reason to share one. */
private fun store(name: String, block: (Connection) -> Unit) = openScratch(name).use { c ->
    c.createInstrumentsTable(); c.createOhlcvTable(); c.createCoverageTable(); c.createSymbologyTable()
    block(c)
}

/** Deterministic bars: close = instrumentId * 1000 + dayOfMonth, so a mislabelled column is obvious. */
private class MarkedFetcher(val calls: MutableList<Pair<DateRange, Set<String>>> = mutableListOf()) : BarFetcher {
    override val source = "test"
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        calls += range to instruments.keys.toSet()
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .flatMap { day -> instruments.values.asSequence().map { id ->
                BarRow(day.atStartOfDay(), id, null, null, null, id * 1000.0 + day.dayOfMonth, 100L) } }
            .toList()
    }
}

class ColumnLabellingTest {
    /**
     * Regression: loadMatrix sorted labels alphabetically while the grid query emits
     * columns ordered by instrument_id. When those disagree every column carries
     * another symbol's prices, with no visible symptom.
     */
    @Test
    fun `column labels follow instrument id order, not alphabetical order`() = store("labels") { c ->
        // deliberately inverted: alphabetically first symbol has the HIGHEST id
        c.registerInstrument(Instrument(300L, "AAA", d("1990-01-01"), null))
        c.registerInstrument(Instrument(200L, "MMM", d("1990-01-01"), null))
        c.registerInstrument(Instrument(100L, "ZZZ", d("1990-01-01"), null))

        val range = r("2024-03-01", "2024-03-05")
        val ids = mapOf("AAA" to 300L, "MMM" to 200L, "ZZZ" to 100L)
        c.ensureBars(MarkedFetcher(), ids, Schema.OHLCV_1D, range)

        val m = c.loadMatrix(listOf("AAA", "MMM", "ZZZ"), range, source = "test")

        assertEquals(listOf("ZZZ", "MMM", "AAA"), m.symbols, "labels must follow instrument_id order")
        for (row in 0 until m.rows) {
            for ((col, sym) in m.symbols.withIndex()) {
                val expectedId = ids.getValue(sym)
                val actual = m.rowMajor[row * m.cols + col]
                assertEquals(expectedId * 1000.0 + m.dates[row].substring(8, 10).toInt(), actual,
                    "value at ($row,$col) belongs to a different instrument than label '$sym'")
            }
        }
    }
}

class CoverageTest {
    @Test
    fun `a held range is never re-fetched`() = store("cov1") { c ->
        c.registerInstrument(Instrument(1L, "X", d("1990-01-01"), null))
        val f = MarkedFetcher()
        val range = r("2024-03-01", "2024-03-31")

        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, range)
        assertEquals(1, f.calls.size, "first ask must fetch")

        f.calls.clear()
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, range)
        assertTrue(f.calls.isEmpty(), "a fully covered range must not touch the vendor")
    }

    @Test
    fun `only the gap is fetched when extending a range`() = store("cov2") { c ->
        c.registerInstrument(Instrument(1L, "X", d("1990-01-01"), null))
        val f = MarkedFetcher()
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, r("2024-03-01", "2024-03-31"))
        f.calls.clear()

        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, r("2024-03-01", "2024-04-30"))
        assertEquals(listOf(r("2024-04-01", "2024-04-30")), f.calls.map { it.first })
    }

    @Test
    fun `a hole between two held ranges is fetched, and only the hole`() = store("cov3") { c ->
        c.registerInstrument(Instrument(1L, "X", d("1990-01-01"), null))
        val f = MarkedFetcher()
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, r("2024-01-01", "2024-01-31"))
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, r("2024-04-01", "2024-04-30"))
        f.calls.clear()

        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, r("2024-01-01", "2024-04-30"))
        assertEquals(listOf(r("2024-02-01", "2024-03-31")), f.calls.map { it.first })
    }

    @Test
    fun `an empty result is recorded so a delisted name is asked about once`() = store("cov4") { c ->
        c.registerInstrument(Instrument(1L, "GONE", d("1990-01-01"), null))
        val empty = object : BarFetcher {
            override val source = "test"
            var calls = 0
            override fun fetch(i: Map<String, Long>, s: Schema, r: DateRange) = emptyList<BarRow>().also { calls++ }
        }
        val range = r("2024-01-01", "2024-01-31")
        c.ensureBars(empty, mapOf("GONE" to 1L), Schema.OHLCV_1D, range)
        c.ensureBars(empty, mapOf("GONE" to 1L), Schema.OHLCV_1D, range)
        assertEquals(1, empty.calls, "'checked, nothing there' must be remembered")
        assertTrue(c.isCovered(Slice("test", 1L, Schema.OHLCV_1D), range))
    }

    @Test
    fun `coverage is per schema, so daily never satisfies a minute request`() = store("cov5") { c ->
        c.registerInstrument(Instrument(1L, "X", d("1990-01-01"), null))
        val f = MarkedFetcher()
        val range = r("2024-03-01", "2024-03-05")
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1D, range)
        f.calls.clear()
        c.ensureBars(f, mapOf("X" to 1L), Schema.OHLCV_1M, range)
        assertEquals(1, f.calls.size, "a different schema is a different slice")
    }

    @Test
    fun `coverage is per source, so one dataset never satisfies another`() = store("cov6") { c ->
        c.registerInstrument(Instrument(1L, "X", d("1990-01-01"), null))
        val a = object : BarFetcher by MarkedFetcher() { override val source = "databento:EQUS.SUMMARY" }
        val b = MarkedFetcher()
        val range = r("2024-03-01", "2024-03-05")
        c.ensureBars(a, mapOf("X" to 1L), Schema.OHLCV_1D, range)
        c.ensureBars(b, mapOf("X" to 1L), Schema.OHLCV_1D, range)
        assertEquals(1, b.calls.size, "a consolidated feed must not satisfy a single-venue request")
    }
}

class IdentityTest {
    @Test
    fun `a ticker resolves to different instruments on different dates`() = store("id1") { c ->
        c.registerInstrument(Instrument(1L, "T", d("2020-01-01"), d("2022-12-31")))
        c.registerInstrument(Instrument(2L, "T", d("2023-01-01"), null))
        assertEquals(1L, c.resolveInstrument("T", d("2021-06-01")))
        assertEquals(2L, c.resolveInstrument("T", d("2024-06-01")))
    }

    @Test
    fun `a date outside every validity window resolves to nothing`() = store("id2") { c ->
        c.registerInstrument(Instrument(1L, "GONE", d("2020-01-01"), d("2023-05-01")))
        assertNull(c.resolveInstrument("GONE", d("2024-01-01")), "post-delisting must be null, not a guess")
    }

    /** Regression: two instruments claiming one ticker used to tie-break arbitrarily. */
    @Test
    fun `ambiguous identity fails loudly rather than picking one`() = store("id3") { c ->
        c.registerInstrument(Instrument(1L, "DUP", d("1990-01-01"), null))
        c.registerInstrument(Instrument(2L, "DUP", d("1990-01-01"), null))
        val e = assertFailsWith<IllegalStateException> { c.resolveInstrument("DUP", d("2024-01-01")) }
        assertContains(e.message!!, "resolves to 2 instruments")
    }

    @Test
    fun `an unresolvable symbol is reported, not silently dropped`() = store("id4") { c ->
        c.registerInstrument(Instrument(1L, "HERE", d("1990-01-01"), null))
        val (hits, misses) = c.resolveUniverse(listOf("HERE", "NOPE"), d("2024-01-01"))
        assertEquals(mapOf("HERE" to 1L), hits)
        assertEquals(listOf("NOPE"), misses)
    }
}
