package aladin.store

import aladin.BarRow
import aladin.DateRange
import aladin.Instrument
import aladin.Schema
import aladin.vendor.BarFetcher
import aladin.vendor.SymbologySource
import java.io.File
import java.nio.file.Files
import java.sql.SQLException
import java.time.LocalDate
import kotlin.test.*

private fun d(s: String) = LocalDate.parse(s)
private fun r(a: String, b: String) = DateRange(d(a), d(b))
private fun live(id: Long, symbol: String) = listOf(Instrument(id, symbol, d("1900-01-01"), null))

/** Deterministic bars: close = instrumentId * 1000 + dayOfMonth, so a mislabelled column shows. */
private class FakeVendor(override val availability: DateRange? = null) : BarFetcher {
    override val source = "test:fake"
    val calls = mutableListOf<Pair<DateRange, Set<String>>>()
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        calls += range to instruments.keys.toSet()
        return generateSequence(range.from) { it.plusDays(1) }
            .takeWhile { !it.isAfter(range.to) }
            .flatMap { day ->
                instruments.values.asSequence().map { id ->
                    val close = id * 1000.0 + day.dayOfMonth
                    BarRow(day.atStartOfDay(), id, close - 1, close + 1, close - 2, close, id * 10L)
                }
            }
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

private fun store(
    name: String,
    sym: SymbologySource? = null,
    availability: DateRange? = null,
    block: (BarStore, FakeVendor) -> Unit,
) {
    val vendor = FakeVendor(availability)
    BarStore(openScratch(name), vendor, sym, source = vendor.source).use { block(it, vendor) }
}

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
        val syms = FakeSymbology(
            mapOf("AAA" to live(300, "AAA"), "MMM" to live(200, "MMM"), "ZZZ" to live(100, "ZZZ"))
        )
        store("s4", syms) { store, _ ->
            val m = store.bars(listOf("AAA", "MMM", "ZZZ"), range)
            assertEquals(listOf("ZZZ", "MMM", "AAA"), m.symbols)
            val ids = mapOf("AAA" to 300L, "MMM" to 200L, "ZZZ" to 100L)
            for (row in 0 until m.rows) for ((col, sym) in m.symbols.withIndex()) {
                val day = m.dates[row].substring(8, 10).toInt()
                assertEquals(
                    ids.getValue(sym) * 1000.0 + day, m[row, col],
                    "value at ($row,$col) does not belong to label '$sym'",
                )
            }
        }
    }

    @Test
    fun `a symbol that did not exist as-of the date is dropped, not carried empty`() {
        val syms = FakeSymbology(
            mapOf(
                "LIVE" to live(10, "LIVE"),
                "GONE" to listOf(Instrument(20, "GONE", d("2020-01-01"), d("2023-05-01"))),
            )
        )
        store("s5", syms) { store, _ ->
            assertEquals(listOf("LIVE"), store.bars(listOf("LIVE", "GONE"), range).symbols)
        }
    }

    @Test
    fun `identity is asked for once per symbol, positive or negative`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A")))
        store("s6", syms) { store, _ ->
            store.bars(listOf("A"), range)
            repeat(2) { runCatching { store.bars(listOf("A", "NOPE"), range) } }
            assertEquals(2, syms.lookups, "each symbol resolves once, misses included")
        }
    }

    @Test
    fun `a read-only store serves what is held and never fetches`() {
        // One connection, two stores: a named in-memory database lives only while a
        // connection to it is open, so seeding through a store that then closes would
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
    fun `asking for a wholly unknown universe fails and names the symbols`() {
        BarStore(openScratch("s8"), fetcher = null, source = "test:fake").use { store ->
            assertContains(
                assertFailsWith<IllegalArgumentException> { store.bars(listOf("NEVERHEARD"), range) }.message!!,
                "NEVERHEARD",
            )
        }
    }

    @Test
    fun `blank and duplicate symbols are rejected or collapsed`() {
        BarStore(openScratch("s9"), fetcher = null, source = "test:fake").use { store ->
            assertFailsWith<IllegalArgumentException> { store.bars(emptyList(), range) }
            assertFailsWith<IllegalArgumentException> { store.bars(listOf("  ", ""), range) }
        }
    }

    @Test
    fun `the frame carries every column, plus the ticker as-of the bar`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A"), "B" to live(20, "B")))
        store("s11", syms) { store, _ ->
            val df = store.frame(listOf("A", "B"), range)
            assertEquals(
                listOf("ts", "symbol", "instrument_id", "open", "high", "low", "close", "volume", "adjusted"),
                df.columnNames(),
            )
            assertEquals(20, df.rowsCount(), "10 bars x 2 instruments, dense")
            // the fake writes close = id*1000 + day, high = close + 1
            val first = df["close"][0] as Double
            assertEquals(first + 1, df["high"][0] as Double, "columns must line up within a row")
            assertTrue(df["symbol"].values().all { it == "A" || it == "B" })
        }
    }

    @Test
    fun `a matrix still asks for a single field`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A")))
        store("s12", syms) { store, _ ->
            val close = store.bars(listOf("A"), range, field = "close")
            val high = store.bars(listOf("A"), range, field = "high")
            assertEquals(close[0, 0] + 1, high[0, 0], "the field argument selects the column")
        }
    }

    /**
     * Regression: asking for a range predating the dataset used to cost a round trip and
     * then throw — and with several gaps in flight, throw after some had already been
     * fetched and committed.
     */
    @Test
    fun `a request is clamped to what the source can actually serve`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A")))
        store("s13", syms, availability = r("2024-03-05", "2099-01-01")) { store, vendor ->
            val m = store.bars(listOf("A"), r("2024-03-01", "2024-03-10"))
            assertEquals(listOf(r("2024-03-05", "2024-03-10")), vendor.calls.map { it.first },
                "the vendor must never be asked for dates it cannot serve")
            assertEquals(6, m.rows)
        }
    }

    @Test
    fun `a range entirely outside what the source serves fetches nothing`() {
        val syms = FakeSymbology(mapOf("A" to live(10, "A")))
        store("s14", syms, availability = r("2025-01-01", "2099-01-01")) { store, vendor ->
            runCatching { store.bars(listOf("A"), r("2024-03-01", "2024-03-10")) }
            assertTrue(vendor.calls.isEmpty(), "nothing serviceable means no request at all")
        }
    }

    /**
     * A hole is a date the grid has where this instrument has no bar. A date no requested
     * instrument traded never enters the grid at all — so closures and holidays are
     * structurally absent rather than filled with invented prices.
     *
     * The calendar therefore comes from the universe, not the market: for liquid names
     * those coincide, for thin ones they do not, and `lookback` counts rows rather than
     * sessions.
     */
    @Test
    fun `a hole is filled, a date nobody traded is not a row at all`() {
        val partial = object : BarFetcher {
            override val source = "test:partial"
            override fun fetch(i: Map<String, Long>, s: Schema, range: DateRange): List<BarRow> {
                val a = i.getValue("A"); val b = i.getValue("B")
                return buildList {
                    for (day in listOf(4, 5, 6, 8).map { LocalDate.of(2024, 3, it) }) {
                        add(BarRow(day.atStartOfDay(), a, null, null, null, 100.0 + day.dayOfMonth, 1L))
                        // B is halted on the 6th; nobody trades on the 7th
                        if (day.dayOfMonth != 6) {
                            add(BarRow(day.atStartOfDay(), b, null, null, null, 200.0 + day.dayOfMonth, 1L))
                        }
                    }
                }
            }
        }
        BarStore(openScratch("holes"), partial, source = partial.source).use { store ->
            store.register(Instrument(1, "A", d("1900-01-01"), null))
            store.register(Instrument(2, "B", d("1900-01-01"), null))
            val range = r("2024-03-04", "2024-03-08")

            val nan = store.bars(listOf("A", "B"), range)
            assertEquals(4, nan.rows, "5 calendar days, but nobody traded on the 7th")
            assertFalse(nan.dates.any { it.startsWith("2024-03-07") }, "a closed day is not a row")
            assertEquals(1, nan.holes, "only B's halt on the 6th counts as a hole")
            assertTrue(nan[2, nan.columnOf("B")].isNaN())

            val filled = store.bars(listOf("A", "B"), range, holes = Holes.FORWARD_FILL)
            assertEquals(4, filled.rows, "filling holes must not invent rows")
            assertEquals(205.0, filled[2, filled.columnOf("B")], "carried from the 5th")

            val dropped = store.bars(listOf("A", "B"), range, holes = Holes.DROP_DATE)
            assertEquals(3, dropped.rows, "the incomplete date goes, for every instrument")
        }
    }

    @Test
    fun `an unknown price field is rejected rather than interpolated into SQL`() {
        store("s10", FakeSymbology(mapOf("A" to live(10, "A")))) { store, _ ->
            assertFailsWith<IllegalArgumentException> {
                store.bars(listOf("A"), range, field = "close; DROP TABLE ohlcv")
            }
        }
    }
}

/**
 * The store keys on instrument_id so a recycled ticker cannot merge two companies into
 * one series — but that guarantee only holds while the vendor's identity layer is right.
 * Databento maps SPCX to one instrument_id straight through the ticker moving from
 * Tuttle Capital's SPAC ETF to SpaceX, so the store faithfully holds a $23 fund and a
 * $150 rocket company as one instrument. These tests cover noticing that from the data.
 */
class IdentityBreakTest {

    /** A quiet ETF, two years of silence, then a different company at 7x the price. */
    private class RecycledTicker : BarFetcher {
        override val source = "test:recycled"
        override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
            val id = instruments.values.first()
            val etf = (0L..5L).map {
                BarRow(d("2024-04-01").plusDays(it).atStartOfDay(), id, null, null, null, 23.2, 100L)
            }
            val newco = (0L..5L).map {
                BarRow(d("2026-06-12").plusDays(it).atStartOfDay(), id, null, null, null, 163.9, 1_600_000L)
            }
            return (etf + newco).filter { !it.ts.toLocalDate().isBefore(range.from) && !it.ts.toLocalDate().isAfter(range.to) }
        }
    }

    @Test
    fun `a long silence followed by a price jump is flagged`() {
        val vendor = RecycledTicker()
        BarStore(openScratch("b1"), vendor, source = vendor.source).use { store ->
            store.register(Instrument(15024, "SPCX", d("1900-01-01"), null))
            store.bars(listOf("SPCX"), r("2024-01-01", "2026-08-01"))

            val breaks = store.identityBreaks()
            assertEquals(1, breaks.size, "743 silent days and a 7x jump is not one instrument")
            val b = breaks.single()
            assertEquals("SPCX", b.symbol)
            assertEquals(d("2024-04-06"), b.lastBefore)
            assertEquals(d("2026-06-12"), b.firstAfter)
            assertTrue(b.ratio > 6.0, "got ${b.ratio}")
        }
    }

    /** Either signal alone is ordinary: a halt explains a gap, a split explains a jump. */
    @Test
    fun `a gap without a price jump is not flagged`() {
        val halted = object : BarFetcher {
            override val source = "test:halt"
            override fun fetch(i: Map<String, Long>, s: Schema, range: DateRange): List<BarRow> {
                val id = i.values.first()
                return listOf(d("2024-01-02"), d("2024-09-02")).map {
                    BarRow(it.atStartOfDay(), id, null, null, null, 50.0, 1L)
                }
            }
        }
        BarStore(openScratch("b2"), halted, source = halted.source).use { store ->
            store.register(Instrument(1, "HALT", d("1900-01-01"), null))
            store.bars(listOf("HALT"), r("2024-01-01", "2024-12-31"))
            assertTrue(store.identityBreaks().isEmpty(), "a suspension is not an identity change")
        }
    }

    @Test
    fun `a price jump without a gap is not flagged`() {
        val split = object : BarFetcher {
            override val source = "test:split"
            override fun fetch(i: Map<String, Long>, s: Schema, range: DateRange): List<BarRow> {
                val id = i.values.first()
                return (0L..5L).map {
                    val day = d("2024-01-02").plusDays(it)
                    BarRow(day.atStartOfDay(), id, null, null, null, if (it < 3) 400.0 else 100.0, 1L)
                }
            }
        }
        BarStore(openScratch("b3"), split, source = split.source).use { store ->
            store.register(Instrument(1, "SPLIT", d("1900-01-01"), null))
            store.bars(listOf("SPLIT"), r("2024-01-01", "2024-01-31"))
            assertTrue(store.identityBreaks().isEmpty(), "a 4-for-1 on consecutive days is a split")
        }
    }
}

/**
 * Read-only mode is the whole of the multi-process story: DuckDB allows **one writer or
 * many readers, never both**. These pin the three ways that shows up.
 */
class ReadOnlyStoreTest {

    private fun tempStore(): String =
        File(Files.createTempDirectory("bars").toFile(), "store.duckdb").path

    @Test
    fun `many readers share one file, and none of them can write`() {
        val path = tempStore()
        BarStore(openDb(path), FakeVendor()).use { writer ->
            writer.register(Instrument(1, "AAA", d("1900-01-01"), null))
            assertEquals(3, writer.bars(listOf("AAA"), r("2024-03-04", "2024-03-06")).rows)
        }   // the exclusive lock is released with the writer

        BarStore.readOnly(path).use { a ->
            BarStore.readOnly(path).use { b ->
                for (reader in listOf(a, b)) {
                    assertEquals(3, reader.bars(listOf("AAA"), r("2024-03-04", "2024-03-06")).rows)
                }
                assertFailsWith<SQLException> { b.register(Instrument(2, "BBB", d("1900-01-01"), null)) }
            }
        }
    }

    @Test
    fun `read-only names the real problem when there is no store in the file`() {
        val path = tempStore()
        openDb(path).use { it.createStatement().execute("CREATE TABLE unrelated (x INT)") }

        // Without the check this surfaces as the driver's "unsuccessful or closed pending
        // query result", which describes the symptom and hides the cause.
        val e = assertFailsWith<IllegalStateException> { BarStore.readOnly(path) }
        assertTrue("ohlcv" in e.message.orEmpty(), e.message.orEmpty())
        assertTrue("writable" in e.message.orEmpty(), e.message.orEmpty())
    }

    @Test
    fun `read-only never conjures the store it cannot find`() {
        val path = tempStore()
        assertFailsWith<SQLException> { BarStore.readOnly(path) }
        assertFalse(File(path).exists(), "read-only must attach, never create")
    }
}
