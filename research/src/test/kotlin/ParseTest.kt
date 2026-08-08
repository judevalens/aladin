import java.time.LocalDate
import java.time.LocalDateTime
import kotlin.test.*

/**
 * Decoding tests, against payloads captured verbatim from the live API. These are the
 * responses the integration actually receives — every past bug here was a decoding
 * bug, and none were reachable while parsing lived inside a method that made an HTTP call.
 */
class OhlcvCsvTest {

    /** Real response: pretty_px + pretty_ts + map_symbols, two symbols, one session. */
    private val real = """
        ts_event,rtype,publisher_id,instrument_id,open,high,low,close,volume,symbol
        2024-08-01T00:00:00.000000000Z,35,90,38,224.370000000,224.480000000,217.020000000,218.360000000,62500996,AAPL
        2024-08-01T00:00:00.000000000Z,35,90,10888,420.785000000,427.460000000,413.090100000,417.110000000,30296400,MSFT
    """.trimIndent()

    private val ids = mapOf("AAPL" to 1L, "MSFT" to 2L)

    @Test
    fun `decodes a real response`() {
        val rows = OhlcvCsv.parse(real, ids).associateBy { it.instrumentId }
        assertEquals(2, rows.size)

        val aapl = rows.getValue(1L)
        assertEquals(LocalDateTime.parse("2024-08-01T00:00"), aapl.ts)
        assertEquals(218.36, aapl.close, "close must be the decimal the server sent, not fixed-point")
        assertEquals(224.37, aapl.open)
        assertEquals(62_500_996L, aapl.volume)
        assertEquals(417.11, rows.getValue(2L).close)
    }

    @Test
    fun `rows are keyed by symbol, because row order is not stable`() {
        val reversed = real.lines().let { listOf(it[0], it[2], it[1]) }.joinToString("\n")
        val a = OhlcvCsv.parse(real, ids).associate { it.instrumentId to it.close }
        val b = OhlcvCsv.parse(reversed, ids).associate { it.instrumentId to it.close }
        assertEquals(a, b, "reordering rows must not change which instrument gets which price")
    }

    @Test
    fun `a symbol outside the request is dropped, not mis-attributed`() {
        val rows = OhlcvCsv.parse(real, mapOf("AAPL" to 1L))
        assertEquals(listOf(1L), rows.map { it.instrumentId })
    }

    /** Regression: without map_symbols there is no symbol column and rows are unattributable. */
    @Test
    fun `a response missing the symbol column is rejected, not guessed at`() {
        val noSymbol = """
            ts_event,rtype,publisher_id,instrument_id,open,high,low,close,volume
            2024-08-01T00:00:00.000000000Z,35,90,38,224.37,224.48,217.02,218.36,62500996
        """.trimIndent()
        val e = assertFailsWith<IllegalArgumentException> { OhlcvCsv.parse(noSymbol, ids) }
        assertContains(e.message!!, "map_symbols")
    }

    @Test
    fun `a header-only response is empty, not an error`() {
        assertTrue(OhlcvCsv.parse(real.lines().first(), ids).isEmpty())
    }

    @Test
    fun `a short response is refused rather than stored as if complete`() {
        OhlcvCsv.assertComplete(real, expected = 2)          // matches
        OhlcvCsv.assertComplete(real, expected = -1)         // unknown, allowed
        val e = assertFailsWith<IllegalStateException> { OhlcvCsv.assertComplete(real, expected = 100) }
        assertContains(e.message!!, "truncated")
    }
}

class SymbologyJsonTest {
    private val from = LocalDate.parse("2024-07-01")
    private val to = LocalDate.parse("2026-08-01")

    /** Real response for a live symbol spanning the whole dataset. */
    private val live = """{"result":{"AAPL":[{"d0":"2024-07-01","d1":"2026-08-01","s":"38"}]},
        "symbols":["AAPL"],"partial":[],"not_found":[],"message":"OK","status":0}"""

    /** Real response for a ticker that never traded. */
    private val missing = """{"result":{"NOTAREALTICKER":[]},"symbols":["NOTAREALTICKER"],
        "partial":[],"not_found":["NOTAREALTICKER"],"message":"Not found","status":2}"""

    @Test
    fun `uses the vendor's instrument id`() {
        val i = SymbologyJson.parse(live, "AAPL", from, to).single()
        assertEquals(38L, i.id)
        assertEquals("AAPL", i.symbol)
    }

    /**
     * The response clips intervals to the query window, so a boundary at the edge says
     * nothing. Treating it as a listing or delisting date invents history the vendor
     * never asserted.
     */
    @Test
    fun `a boundary at the edge of the window is not a listing or delisting date`() {
        val i = SymbologyJson.parse(live, "AAPL", from, to).single()
        assertEquals(DATASET_FLOOR, i.validFrom, "start at the window edge means 'at or before'")
        assertNull(i.validTo, "end at the window edge means 'still current', not delisted")
    }

    @Test
    fun `a boundary strictly inside the window is real`() {
        val delisted = """{"result":{"SIVB":[{"d0":"2024-09-02","d1":"2025-03-10","s":"77"}]},
            "not_found":[],"status":0}"""
        val i = SymbologyJson.parse(delisted, "SIVB", from, to).single()
        assertEquals(LocalDate.parse("2024-09-02"), i.validFrom)
        assertEquals(LocalDate.parse("2025-03-10"), i.validTo)
    }

    /** A recycled ticker is several intervals — the reason identity is not the symbol. */
    @Test
    fun `a recycled ticker yields one instrument per interval`() {
        val recycled = """{"result":{"T":[
            {"d0":"2024-09-01","d1":"2025-01-01","s":"11"},
            {"d0":"2025-06-01","d1":"2025-12-01","s":"22"}]},"not_found":[],"status":0}"""
        val all = SymbologyJson.parse(recycled, "T", from, to)
        assertEquals(listOf(11L, 22L), all.map { it.id })
        assertEquals(LocalDate.parse("2025-01-01"), all[0].validTo)
        assertEquals(LocalDate.parse("2025-06-01"), all[1].validFrom)
    }

    @Test
    fun `not_found is an empty history, not an error`() {
        assertTrue(SymbologyJson.parse(missing, "NOTAREALTICKER", from, to).isEmpty())
    }
}
