package aladin.vendor

import aladin.BarRow
import aladin.DateRange
import aladin.Schema
import java.time.LocalDate
import kotlin.test.*

private fun r(a: String, b: String) = DateRange(LocalDate.parse(a), LocalDate.parse(b))

/** A priceable fetcher whose cost we control, so the gate is testable without spending. */
private class Priced(private val cost: Double) : PricedFetcher {
    override val source = "test"
    var fetches = 0
    override fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange) = cost
    override fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange) = 42L
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange) =
        emptyList<BarRow>().also { fetches++ }
}

class BudgetTest {
    private val range = r("2024-03-01", "2024-03-31")
    private val one = mapOf("X" to 1L)

    @Test
    fun `small requests proceed without asking`() {
        val inner = Priced(0.01)
        BudgetedFetcher(inner, autoApproveUnder = 0.10,
            approver = { _, _ -> fail("must not prompt below the threshold") })
            .fetch(one, Schema.OHLCV_1D, range)
        assertEquals(1, inner.fetches)
    }

    @Test
    fun `above the threshold it asks, and yes proceeds`() {
        val inner = Priced(5.0)
        var asked = false
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, approver = { _, _ -> asked = true; true })
        gated.fetch(one, Schema.OHLCV_1D, range)
        assertTrue(asked)
        assertEquals(5.0, gated.spentUsd)
    }

    @Test
    fun `a decline means nothing is fetched and nothing is recorded as spent`() {
        val inner = Priced(5.0)
        val gated = BudgetedFetcher(inner, autoApproveUnder = 0.10, approver = { _, _ -> false })
        assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }
        assertEquals(0, inner.fetches)
        assertEquals(0.0, gated.spentUsd)
    }

    @Test
    fun `the hard ceiling is not even offered for approval`() {
        val inner = Priced(100.0)
        val gated = BudgetedFetcher(inner, hardCeiling = 25.0,
            approver = { _, _ -> fail("must not prompt above the hard ceiling") })
        assertContains(
            assertFailsWith<IllegalStateException> { gated.fetch(one, Schema.OHLCV_1D, range) }.message!!,
            "hard ceiling",
        )
        assertEquals(0, inner.fetches)
    }

    @Test
    fun `an unpriceable request is refused rather than bought blind`() {
        val blind = object : PricedFetcher {
            override val source = "test"
            var fetches = 0
            override fun estimateCostUsd(s: Collection<String>, sc: Schema, r: DateRange): Double =
                error("pricing endpoint down")
            override fun recordCount(s: Collection<String>, sc: Schema, r: DateRange) = -1L
            override fun fetch(i: Map<String, Long>, s: Schema, r: DateRange) =
                emptyList<BarRow>().also { fetches++ }
        }
        assertFailsWith<IllegalStateException> {
            BudgetedFetcher(blind, approver = { _, _ -> true }).fetch(one, Schema.OHLCV_1D, range)
        }
        assertEquals(0, blind.fetches)
    }

    /** A ceiling below the auto-approve threshold would refuse everything, silently. */
    @Test
    fun `a contradictory budget is rejected at construction`() {
        assertFailsWith<IllegalArgumentException> {
            BudgetedFetcher(Priced(1.0), autoApproveUnder = 10.0, hardCeiling = 1.0)
        }
    }
}
