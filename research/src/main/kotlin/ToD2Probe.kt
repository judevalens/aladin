import org.jetbrains.kotlinx.dataframe.api.colsOf

fun main() {
    openDb().use { conn ->
        val wide = conn.frame("""
            SELECT ts, MAX(close) FILTER (symbol='AMD')  AS AMD,
                       MAX(close) FILTER (symbol='AVGO') AS AVGO,
                       MAX(close) FILTER (symbol='NVDA') AS NVDA
            FROM bars GROUP BY ts ORDER BY ts LIMIT 4""")
        println(wide)

        println("\nconvertToMultik()  (auto, numeric columns only)")
        println(wide.convertToMultik())

        println("\nconvertToMultik { colsOf<Double>() }  (typed selector)")
        println(wide.convertToMultik { colsOf<Double>() })
    }
}
