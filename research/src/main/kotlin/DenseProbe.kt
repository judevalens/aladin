import java.time.LocalDate

/** A ragged universe — AMD halts a day, a new listing starts late — made dense in SQL. */
fun main() {
    openDb().use { c ->
        c.createStatement().use {
            it.execute("DROP TABLE IF EXISTS ohlcv"); it.execute("DROP TABLE IF EXISTS coverage")
        }
        c.createOhlcvTable(); c.createCoverageTable()

        val days = (2..6).map { LocalDate.parse("2024-03-0$it") }
        c.prepareStatement("INSERT INTO ohlcv VALUES (?,?,?,?,?,?,?,?,?,?)").use { ps ->
            fun put(id: Long, d: LocalDate, px: Double) {
                ps.setString(1, "fake"); ps.setLong(2, id); ps.setString(3, "ohlcv-1d")
                ps.setTimestamp(4, java.sql.Timestamp.valueOf(d.atStartOfDay()))
                ps.setObject(5, px); ps.setObject(6, px); ps.setObject(7, px); ps.setObject(8, px)
                ps.setObject(9, 100L); ps.setBoolean(10, false); ps.addBatch()
            }
            days.forEach { put(1L, it, 100.0) }                       // NVDA: complete
            days.filter { it.dayOfMonth != 4 }.forEach { put(3L, it, 50.0) }   // AMD: halted the 4th
            days.filter { it.dayOfMonth >= 5 }.forEach { put(7L, it, 20.0) }   // IPO mid-range
            ps.executeBatch()
        }

        println("raw store — ragged, which is correct for a long table:")
        println(c.frame("SELECT instrument_id, count(*) AS bars FROM ohlcv GROUP BY 1 ORDER BY 1"))

        println("\nrectangular by construction — CROSS JOIN the grid, LEFT JOIN the bars:")
        println(c.frame("""
            WITH cal  AS (SELECT DISTINCT ts FROM ohlcv WHERE schema = 'ohlcv-1d'),
                 inst AS (SELECT unnest([1, 3, 7]) AS instrument_id)
            SELECT c.ts, i.instrument_id, o.close
            FROM cal c CROSS JOIN inst i
            LEFT JOIN ohlcv o
                   ON o.ts = c.ts AND o.instrument_id = i.instrument_id AND o.schema = 'ohlcv-1d'
            ORDER BY c.ts, i.instrument_id"""))
    }
}
