package aladin.store

import aladin.BarRow
import aladin.DateRange
import aladin.Instrument
import aladin.Schema
import aladin.vendor.BarFetcher
import aladin.vendor.SymbologySource
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

    @Test
    fun `an unknown price field is rejected rather than interpolated into SQL`() {
        store("s10", FakeSymbology(mapOf("A" to live(10, "A")))) { store, _ ->
            assertFailsWith<IllegalArgumentException> {
                store.bars(listOf("A"), range, field = "close; DROP TABLE ohlcv")
            }
        }
    }
}
