package aladin

import java.time.LocalDate
import kotlin.test.*

class DomainTest {
    @Test
    fun `a backwards range is rejected at construction`() {
        assertFailsWith<IllegalArgumentException> {
            DateRange(LocalDate.parse("2024-03-10"), LocalDate.parse("2024-03-01"))
        }
    }

    @Test
    fun `day counts are inclusive at both ends`() {
        assertEquals(1, DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-03-01")).days)
        assertEquals(31, DateRange(LocalDate.parse("2024-03-01"), LocalDate.parse("2024-03-31")).days)
    }

    /** The cost question, computable rather than a table in a doc. */
    @Test
    fun `schema carries the bar counts that billing tracks`() {
        assertEquals(1, Schema.OHLCV_1D.barsPerSession)
        assertEquals(390, Schema.OHLCV_1M.barsPerSession)
        assertEquals(23_400.0, Schema.OHLCV_1S.vsDaily)
        assertEquals(1_260_000L, Schema.OHLCV_1D.rowsFor(instruments = 500, sessions = 2520))
        assertEquals(Schema.OHLCV_1D, Schema.of("ohlcv-1d"))
        assertFailsWith<IllegalStateException> { Schema.of("ohlcv-1y") }
    }

    @Test
    fun `an index is not tradeable`() {
        assertTrue(InstrumentType.EQUITY.tradeable)
        assertFalse(InstrumentType.INDEX.tradeable)
        assertFalse(InstrumentType.UNKNOWN.tradeable)
    }

    @Test
    fun `validity windows are inclusive and an open end means still current`() {
        val closed = Instrument(1, "X", LocalDate.parse("2020-01-01"), LocalDate.parse("2023-05-01"))
        assertTrue(closed.coversAsOf(LocalDate.parse("2020-01-01")))
        assertTrue(closed.coversAsOf(LocalDate.parse("2023-05-01")))
        assertFalse(closed.coversAsOf(LocalDate.parse("2023-05-02")))

        val open = Instrument(2, "Y", LocalDate.parse("2020-01-01"), null)
        assertTrue(open.coversAsOf(LocalDate.parse("2099-01-01")))
    }

    @Test
    fun `a slice needs a scope, and an instrument a symbol`() {
        assertFailsWith<IllegalArgumentException> { Slice("", 1L, Schema.OHLCV_1D) }
        assertFailsWith<IllegalArgumentException> { Instrument(1, "  ", LocalDate.parse("2020-01-01"), null) }
    }

    @Test
    fun `symbols are trimmed and de-duplicated, and an empty request is rejected`() {
        assertEquals(listOf("AAPL", "MSFT"), normalizeSymbols(listOf(" AAPL ", "MSFT", "AAPL", "")))
        assertFailsWith<IllegalArgumentException> { normalizeSymbols(emptyList()) }
        assertFailsWith<IllegalArgumentException> { normalizeSymbols(listOf(" ", "")) }
    }
}
