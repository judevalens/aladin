import java.time.LocalDate

private fun d(s: String) = LocalDate.parse(s)
private fun r(a: String, b: String) = DateRange(d(a), d(b))

fun main() {
    val nvda = Slice("databento", "NVDA", "ohlcv-1d")
    openDb().use { conn ->
        conn.createStatement().use { it.execute("DROP TABLE IF EXISTS coverage") }
        conn.createCoverageTable()

        fun show(label: String, want: DateRange) {
            val covered = conn.isCovered(nvda, want)
            val missing = conn.missingRanges(nvda, want)
            println("  ${label.padEnd(40)} covered=${covered.toString().padEnd(5)} missing=$missing")
        }

        println("empty ledger:")
        show("want 2024-01-01..2024-12-31", r("2024-01-01", "2024-12-31"))

        println("\nafter fetching 2024-01-01..2024-06-30:")
        conn.recordCoverage(nvda, r("2024-01-01", "2024-06-30"), rows = 124)
        show("want 2024-01-01..2024-06-30  (exact)", r("2024-01-01", "2024-06-30"))
        show("want 2024-03-01..2024-04-30  (inside)", r("2024-03-01", "2024-04-30"))
        show("want 2024-01-01..2024-12-31  (extends)", r("2024-01-01", "2024-12-31"))

        println("\nafter also fetching 2024-10-01..2024-12-31 (leaves a hole):")
        conn.recordCoverage(nvda, r("2024-10-01", "2024-12-31"), rows = 64)
        show("want 2024-01-01..2024-12-31", r("2024-01-01", "2024-12-31"))

        println("\nafter filling the hole 2024-07-01..2024-09-30:")
        conn.recordCoverage(nvda, r("2024-07-01", "2024-09-30"), rows = 64)
        show("want 2024-01-01..2024-12-31", r("2024-01-01", "2024-12-31"))

        conn.createStatement().use { st ->
            st.executeQuery("SELECT start_date, end_date, rows FROM coverage ORDER BY start_date").use { rs ->
                println("\n  ledger rows (abutting ranges merged on write):")
                while (rs.next()) println("    ${rs.getDate(1)} .. ${rs.getDate(2)}   ${rs.getLong(3)} bars")
            }
        }

        println("\nknown-empty (delisted): record rows=0 so it is never re-requested")
        val sivb = Slice("databento", "SIVB", "ohlcv-1d")
        conn.recordCoverage(sivb, r("2024-01-01", "2024-12-31"), rows = 0)
        println("  SIVB 2024 covered=${conn.isCovered(sivb, r("2024-01-01", "2024-12-31"))}  " +
                "missing=${conn.missingRanges(sivb, r("2024-01-01", "2024-12-31"))}")
    }
}
