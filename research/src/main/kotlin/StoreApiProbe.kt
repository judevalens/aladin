import java.time.LocalDate

/** What a strategy sees: no SQL, both return shapes, holes handled explicitly. */
fun main() {
    openDb().use { conn ->
        conn.seedFixture()
        val range = DateRange(LocalDate.parse("2024-01-02"), LocalDate.parse("2024-06-28"))
        val syms = listOf("NVDA", "AMD", "AVGO")

        val m = conn.loadMatrix(syms, range, source = "bars")
        println("loadMatrix -> BarMatrix ${m.rows} x ${m.cols}  holes=${m.holes}  ${m.symbols}")
        println("  first bar: " + m.symbols.indices.joinToString("  ") { "%.2f".format(m.rowMajor[it]) })
        println("  -> multik ${m.nd().shape.toList()},  colMajor ${m.colMajor().size}")

        println("\nloadFrame -> DataFrame (same query, boxed and printable)")
        println(conn.loadFrame(syms, DateRange(range.from, range.from.plusDays(2)), source = "bars"))

        println("\nas-of protection: a symbol that did not exist then is not silently dropped")
        conn.registerInstrument(Instrument(99L, "IPO24", LocalDate.parse("2024-06-01"), null))
        val (hits, misses) = conn.resolveUniverse(syms + "IPO24", LocalDate.parse("2024-01-02"))
        println("  as-of 2024-01-02  resolved=${hits.keys}  unresolved=$misses")

        println("\nhole policies on a deliberately gapped universe (scratch copy):")
        // gap a COPY — never mutate the shared fixture, VerifyKt reads it
        conn.createStatement().use {
            it.execute("DROP TABLE IF EXISTS ohlcv_gapped")
            it.execute("CREATE TABLE ohlcv_gapped AS SELECT * FROM ohlcv WHERE source='bars'")
            it.execute("UPDATE ohlcv_gapped SET source='gapped'")
            it.execute("INSERT INTO ohlcv SELECT * FROM ohlcv_gapped")
            it.execute("DROP TABLE ohlcv_gapped")
            it.execute("DELETE FROM ohlcv WHERE source='gapped' AND instrument_id=1 AND ts IN (SELECT ts FROM ohlcv WHERE source='gapped' ORDER BY ts LIMIT 3)")
        }
        listOf(1L, 2L, 3L).forEach { conn.recordCoverage(Slice("gapped", it, Schema.OHLCV_1D), range) }
        for (h in Holes.entries) {
            val g = conn.loadMatrix(syms, range, source = "gapped", holes = h)
            println("  ${h.name.padEnd(13)} rows=${g.rows}  holes=${g.holes}  " +
                    "first AMD value=${"%.2f".format(g.rowMajor[0])}")
        }
    }
}
