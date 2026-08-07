/** DataFrame as the exploration surface over the DuckDB store. */

import org.jetbrains.kotlinx.dataframe.api.*

fun main() {
    openDb().use { conn ->
        println("=== schema ===")
        println(conn.frame("SELECT * FROM bars LIMIT 1").schema())

        println("\n=== head ===")
        println(conn.frame("SELECT * FROM bars ORDER BY ts LIMIT 5"))

        println("\n=== describe() — DataFrame does the stats ===")
        println(conn.frame("SELECT symbol, close FROM bars").describe())

        println("\n=== per-symbol summary, aggregated in DataFrame ===")
        println(
            conn.frame("SELECT symbol, ts, close FROM bars")
                .groupBy("symbol")
                .aggregate {
                    count() into "bars"
                    min("close") into "low"
                    max("close") into "high"
                    mean("close") into "avg"
                }
        )

        println("\n=== or aggregated in SQL, if the data is big ===")
        println(conn.frame("""
            SELECT symbol, count(*) AS bars, min(close) AS low, max(close) AS high,
                   round(avg(close), 2) AS avg
            FROM bars GROUP BY symbol ORDER BY symbol"""))

        println("\n=== wide, ready to plot or convert ===")
        val wide = conn.frame("""
            SELECT ts, MAX(close) FILTER (symbol='AMD')  AS AMD,
                       MAX(close) FILTER (symbol='AVGO') AS AVGO,
                       MAX(close) FILTER (symbol='NVDA') AS NVDA
            FROM bars GROUP BY ts ORDER BY ts DESC LIMIT 5""")
        println(wide)
        println("-> multik ${wide.convertToMultik { colsOf<Double>() }.shape.toList()}")
    }
}
