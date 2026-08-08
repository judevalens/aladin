import java.time.LocalDate
import kotlin.test.*

private fun r(a: String, b: String) = DateRange(LocalDate.parse(a), LocalDate.parse(b))

/** A priceable fetcher whose cost we control, so the gate can be tested without spending. */
private class PricedFake(private val cost: Double) : PricedFetcher {
    override val source = "test"
    var fetches = 0
    override fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange) = cost
    override fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange) = 42L
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        fetches++
        return listOf(BarRow(range.from.atStartOfDay(), instruments.values.first(), null, null, null, 1.0, 1L))
    }
}

class BudgetTest {
    private val range = r("2024-03-01", "2024-03-31")
    private val one = mapOf("X" to 1L)

    @Test
    fun `small requests go through without asking`() {
        val inner = PricedFake(0.01)
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10,
            approver = { _, _ -> fail("must not prompt below the auto-approve threshold") })
        gated.fetch(one, Schema.OHLCV_1D, range)
        assertEquals(1, inner.fetches)
        assertEquals(0.01, gated.spentUsd)
    }

    @Test
    fun `above the threshold it asks, and a yes proceeds`() {
        val inner = PricedFake(5.0)
        var asked = false
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10,
            approver = { _, _ -> asked = true; true })
        gated.fetch(one, Schema.OHLCV_1D, range)
        assertTrue(asked)
        assertEquals(1, inner.fetches)
    }

    @Test
    fun `a no means nothing is fetched`() {
        val inner = PricedFake(5.0)
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, approver = { _, _ -> false })
        assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }
        assertEquals(0, inner.fetches, "a declined request must not reach the vendor")
        assertEquals(0.0, gated.spentUsd)
    }

    @Test
    fun `the hard ceiling cannot be approved past`() {
        val inner = PricedFake(100.0)
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, hardCeiling = 25.0,
            approver = { _, _ -> fail("must not even offer approval above the hard ceiling") })
        val e = assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }
        assertContains(e.message!!, "hard ceiling")
        assertEquals(0, inner.fetches)
    }

    @Test
    fun `a request that cannot be priced is refused rather than fetched blind`() {
        val blind = object : PricedFetcher {
            override val source = "test"
            var fetches = 0
            override fun estimateCostUsd(s: Collection<String>, sc: Schema, r: DateRange): Double =
                error("pricing endpoint down")
            override fun recordCount(s: Collection<String>, sc: Schema, r: DateRange) = -1L
            override fun fetch(i: Map<String, Long>, s: Schema, r: DateRange) =
                emptyList<BarRow>().also { fetches++ }
        }
        val gated = BudgetedFetcher(blind, approver = { _, _ -> true })
        assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }
        assertEquals(0, blind.fetches)
    }

    /** The cheapest rail: coverage means the gate is never even consulted for held data. */
    @Test
    fun `held data costs nothing because it is never priced`() = openScratch("budget").use { c ->
        c.createInstrumentsTable(); c.createOhlcvTable(); c.createCoverageTable()
        c.registerInstrument(Instrument(1L, "X", LocalDate.parse("1990-01-01"), null))
        val inner = PricedFake(5.0)
        var prompts = 0
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, approver = { _, _ -> prompts++; true })

        c.ensureBars(gated, one, Schema.OHLCV_1D, range)
        assertEquals(1, prompts, "the first fetch is priced and approved")

        c.ensureBars(gated, one, Schema.OHLCV_1D, range)
        assertEquals(1, prompts, "a covered range must not be priced, prompted for, or fetched")
        assertEquals(1, inner.fetches)
    }
}
