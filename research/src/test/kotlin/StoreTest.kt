import java.time.LocalDate
import kotlin.test.*

private fun d(s: String) = LocalDate.parse(s)
private fun r(a: String, b: String) = DateRange(d(a), d(b))

/** Deterministic bars: close = instrumentId * 1000 + dayOfMonth, so a mislabelled column shows. */
private class FakeVendor : BarFetcher {
    override val source = "test:fake"
    val calls = mutableListOf<Pair<DateRange, Set<String>>>()
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        calls += range to instruments.keys.toSet()
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .flatMap { day -> instruments.values.asSequence().map { id ->
                BarRow(day.atStartOfDay(), id, null, null, null, id * 1000.0 + day.dayOfMonth, 1L) } }
            .toList()
    }
}

private class FakeSymbology(private val known: Map<String, List<Instrument>>) : SymbologySource {
    override val source = "test:fake"
    var lookups = 0
    override fun history(symbol: String): List<Instrument> {
        lookups++
        return known[symbol].orEmpty()
    }
}

private fun store(name: String, sym: SymbologySource? = null, block: (BarStore, FakeVendor) -> Unit) {
    val vendor = FakeVendor()
    BarStore(openScratch(name), vendor, sym, source = vendor.source).use { block(it, vendor) }
}

private fun live(id: Long, symbol: String) =
    listOf(Instrument(id, symbol, d("1900-01-01"), null))

class BarStoreTest {
    private val range = r("2024-03-01", "2024-03-10")

    @Test
    fun `one call fetches what is missing and returns a matrix`() =
        store("s1", FakeSymbology(mapOf("A" to live(10, "A"), "B" to live(20, "B")))) { store, vendor ->
            val m = store.bars(listOf("A", "B"), range)
            assertEquals(10, m.rows)
            assertEquals(listOf("A", "B"), m.symbols)
            assertEquals(1, vendor.calls.size)
        }

    @Test
    fun `a second call for held data touches neither vendor nor wallet`() =
        store("s2", FakeSymbology(mapOf("A" to live(10, "A")))) { store, vendor ->
            store.bars(listOf("A"), range)
            vendor.calls.clear()
            store.bars(listOf("A"), range)
            assertTrue(vendor.calls.isEmpty(), "a held range must not be re-fetched")
        }

    @Test
    fun `extending a range fetches only the extension`() =
        store("s3", FakeSymbology(mapOf("A" to live(10, "A")))) { store, vendor ->
            store.bars(listOf("A"), r("2024-03-01", "2024-03-10"))
            vendor.calls.clear()
            store.bars(listOf("A"), r("2024-03-01", "2024-03-20"))
            assertEquals(listOf(r("2024-03-11", "2024-03-20")), vendor.calls.map { it.first })
        }

    /** Regression: labels were sorted alphabetically while the grid orders by instrument_id. */
    @Test
    fun `column labels follow instrument id order and carry their own values`() {
        val syms = FakeSymbology(mapOf("AAA" to live(300, "AAA"), "MMM" to live(200, "MMM"), "ZZZ" to live(100, "ZZZ")))
        store("s4", syms) { store, _ ->
            val m = store.bars(listOf("AAA", "MMM", "ZZZ"), range)
            assertEquals(listOf("ZZZ", "MMM", "AAA"), m.symbols)
            val ids = mapOf("AAA" to 300L, "MMM" to 200L, "ZZZ" to 100L)
            for (row in 0 until m.rows) for ((col, sym) in m.symbols.withIndex()) {
                val day = m.dates[row].substring(8, 10).toInt()
                assertEquals(ids.getValue(sym) * 1000.0 + day, m.rowMajor[row * m.cols + col],
                    "value at ($row,$col) does not belong to label '$sym'")
            }
        }
    }

    @Test
    fun `a symbol that did not exist as-of the date is dropped, not carried empty`() {
        val syms = FakeSymbology(mapOf(
            "LIVE" to live(10, "LIVE"),
            "GONE" to listOf(Instrument(20, "GONE", d("2020-01-01"), d("2023-05-01"))),
        ))
        store("s5", syms) { store, _ ->
            val m = store.bars(listOf("LIVE", "GONE"), range)
            assertEquals(listOf("LIVE"), m.symbols, "a delisted name must not become an empty column")
        }
    }

    @Test
    fun `identity is asked for once per symbol, positive or negative`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A")))     // "NOPE" is unknown
        store("s6", syms) { store, _ ->
            store.bars(listOf("A"), range)
            runCatching { store.bars(listOf("A", "NOPE"), range) }
            runCatching { store.bars(listOf("A", "NOPE"), range) }
            assertEquals(2, syms.lookups, "each symbol is resolved once, misses included")
        }
    }

    @Test
    fun `a read-only store serves what is held and never fetches`() {
        // one connection, two stores. A named in-memory database lives only as long as
        // a connection to it is open, so seeding through a store that then closes would
        // take the data with it.
        openScratch("s7").use { conn ->
            val vendor = FakeVendor()
            val syms = FakeSymbology(mapOf("A" to live(10, "A")))
            BarStore(conn, vendor, syms, source = vendor.source).bars(listOf("A"), range)

            val m = BarStore(conn, fetcher = null, source = vendor.source).bars(listOf("A"), range)
            assertEquals(10, m.rows, "held data is served without a fetcher")
            assertEquals(1, vendor.calls.size, "the read-only store must not have fetched")
        }
    }

    @Test
    fun `asking for a wholly unknown universe fails rather than returning nothing`() {
        BarStore(openScratch("s9"), fetcher = null, source = "test:fake").use { store ->
            val e = assertFailsWith<IllegalArgumentException> { store.bars(listOf("NEVERHEARD"), range) }
            assertContains(e.message!!, "NEVERHEARD", message = "the error must name what could not be resolved")
        }
    }

    @Test
    fun `holes are reported and can be filled by policy`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A"), "B" to live(20, "B")))
        store("s8", syms) { store, _ ->
            store.bars(listOf("A", "B"), range)
            assertEquals(0, store.bars(listOf("A", "B"), range).holes)
        }
    }
}

class BudgetTest {
    private val range = r("2024-03-01", "2024-03-31")
    private val one = mapOf("X" to 1L)

    private class Priced(private val cost: Double) : PricedFetcher {
        override val source = "test"
        var fetches = 0
        override fun estimateCostUsd(s: Collection<String>, sc: Schema, r: DateRange) = cost
        override fun recordCount(s: Collection<String>, sc: Schema, r: DateRange) = 42L
        override fun fetch(i: Map<String, Long>, s: Schema, r: DateRange) =
            emptyList<BarRow>().also { fetches++ }
    }

    @Test
    fun `small requests proceed without asking`() {
        val inner = Priced(0.01)
        BudgetedFetcher(inner, autoApproveUnder = 0.10,
            approver = { _, _ -> fail("must not prompt below the threshold") })
            .fetch(one, Schema.OHLCV_1D, range)
        assertEquals(1, inner.fetches)
    }

    @Test
    fun `a decline means nothing is fetched`() {
        val inner = Priced(5.0)
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, approver = { _, _ -> false })
        assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }
        assertEquals(0, inner.fetches)
        assertEquals(0.0, gated.spentUsd)
    }

    @Test
    fun `the hard ceiling is not offered for approval`() {
        val inner = Priced(100.0)
        val gated = BudgetedFetcher(inner, hardCeiling = 25.0,
            approver = { _, _ -> fail("must not prompt above the hard ceiling") })
        assertContains(assertFailsWith<IllegalStateException> {
            gated.fetch(one, Schema.OHLCV_1D, range)
        }.message!!, "hard ceiling")
        assertEquals(0, inner.fetches)
    }

    @Test
    fun `an unpriceable request is refused rather than bought blind`() {
        val blind = object : PricedFetcher {
            override val source = "test"
            var fetches = 0
            override fun estimateCostUsd(s: Collection<String>, sc: Schema, r: DateRange): Double =
                error("pricing down")
            override fun recordCount(s: Collection<String>, sc: Schema, r: DateRange) = -1L
            override fun fetch(i: Map<String, Long>, s: Schema, r: DateRange) =
                emptyList<BarRow>().also { fetches++ }
        }
        assertFailsWith<IllegalStateException> {
            BudgetedFetcher(blind, approver = { _, _ -> true }).fetch(one, Schema.OHLCV_1D, range)
        }
        assertEquals(0, blind.fetches)
    }
}

class IdentityTest {
    @Test
    fun `a ticker resolves to different instruments on different dates`() = openScratch("i1").use { c ->
        c.createInstrumentsTable()
        c.registerInstrument(Instrument(1L, "T", d("2020-01-01"), d("2022-12-31")))
        c.registerInstrument(Instrument(2L, "T", d("2023-01-01"), null))
        assertEquals(1L, c.resolveInstrument("T", d("2021-06-01")))
        assertEquals(2L, c.resolveInstrument("T", d("2024-06-01")))
    }

    @Test
    fun `outside every window resolves to nothing`() = openScratch("i2").use { c ->
        c.createInstrumentsTable()
        c.registerInstrument(Instrument(1L, "GONE", d("2020-01-01"), d("2023-05-01")))
        assertNull(c.resolveInstrument("GONE", d("2024-01-01")))
    }

    /** Regression: two instruments claiming one ticker used to tie-break silently. */
    @Test
    fun `ambiguous identity fails loudly`() = openScratch("i3").use { c ->
        c.createInstrumentsTable()
        c.registerInstrument(Instrument(1L, "DUP", d("1990-01-01"), null))
        c.registerInstrument(Instrument(2L, "DUP", d("1990-01-01"), null))
        assertContains(assertFailsWith<IllegalStateException> {
            c.resolveInstrument("DUP", d("2024-01-01"))
        }.message!!, "resolves to 2 instruments")
    }
}
