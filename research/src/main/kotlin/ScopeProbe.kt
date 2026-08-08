import java.time.LocalDate

/** Two datasets, same vendor, same instrument, same bar — different scope. What does the store do? */
private class FakeDataset(private val ds: String, private val vol: Long, private val close: Double) : BarFetcher {
    override val source = "databento:$ds"             // scope, not vendor
    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange) =
        listOf(BarRow(LocalDate.parse("2024-08-01").atStartOfDay(), 1L, null, null, null, close, vol))
}

fun main() = openScratch("scope").use { c ->
    c.createOhlcvTable(); c.createCoverageTable(); c.createInstrumentsTable()
    c.registerInstrument(Instrument(1L, "AAPL", LocalDate.parse("1990-01-01"), null))
    val day = DateRange(LocalDate.parse("2024-08-01"), LocalDate.parse("2024-08-01"))

    c.ensureBars(FakeDataset("EQUS.SUMMARY", 62_500_996, 218.36), mapOf("AAPL" to 1L), Schema.OHLCV_1D, day)
    println("after EQUS.SUMMARY:")
    c.createStatement().use { st -> st.executeQuery("SELECT source, close, volume FROM ohlcv").use { rs ->
        while (rs.next()) println("  source=${rs.getString(1)}  close=${rs.getDouble(2)}  volume=${"%,d".format(rs.getLong(3))}") } }

    // a later fetch from a DIFFERENT dataset, same vendor
    c.createStatement().use { it.execute("DELETE FROM coverage") }      // force a re-fetch
    c.ensureBars(FakeDataset("XNAS.ITCH", 21_277_576, 219.65), mapOf("AAPL" to 1L), Schema.OHLCV_1D, day)
    println("\nafter XNAS.ITCH (Nasdaq-only) is fetched into the same store:")
    c.createStatement().use { st -> st.executeQuery("SELECT source, close, volume FROM ohlcv").use { rs ->
        while (rs.next()) println("  source=${rs.getString(1)}  close=${rs.getDouble(2)}  volume=${"%,d".format(rs.getLong(3))}") } }
    println("\n  -> two rows, distinguishable. Coverage is per-scope too, so holding")
    println("     EQUS.SUMMARY never satisfies a request for XNAS.ITCH.")
}
