/**
 * The coverage ledger: which (symbol, schema, date range) slices we already hold.
 *
 * This cannot be derived from the bars table. A missing row is ambiguous — weekend,
 * holiday, trading halt, pre-IPO, post-delisting, or genuinely not fetched. Only a
 * record of what was *requested* distinguishes them.
 *
 * Ranges are stored merged and inclusive on both ends, so the common question —
 * "do I have all of this?" — is a single EXISTS. Gap-finding for partial coverage is
 * interval arithmetic in Kotlin, because a symbol's ledger is a handful of rows and
 * the logic is far clearer in code than in recursive SQL.
 */

import java.sql.Connection
import java.sql.Date
import java.time.LocalDate

data class DateRange(val from: LocalDate, val to: LocalDate) {
    init { require(!from.isAfter(to)) { "empty range: $from..$to" } }
    val days get() = java.time.temporal.ChronoUnit.DAYS.between(from, to) + 1
    override fun toString() = "$from..$to"
}

data class Slice(val source: String, val symbol: String, val schema: String)

fun Connection.createCoverageTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS coverage (
          source      VARCHAR NOT NULL,      -- 'databento' | 'alpaca'
          symbol      VARCHAR NOT NULL,
          schema      VARCHAR NOT NULL,      -- 'ohlcv-1d' | 'ohlcv-1m'
          start_date  DATE    NOT NULL,      -- inclusive
          end_date    DATE    NOT NULL,      -- inclusive
          rows        BIGINT  NOT NULL,      -- 0 is meaningful: checked, legitimately empty
          fetched_at  TIMESTAMP NOT NULL DEFAULT now()
        )""")
}

/** Every stored range for this slice that touches [want], clipped to it, ordered. */
private fun Connection.clippedRanges(s: Slice, want: DateRange): List<DateRange> =
    prepareStatement("""
        SELECT greatest(start_date, ?) AS s, least(end_date, ?) AS e
        FROM coverage
        WHERE source = ? AND symbol = ? AND schema = ?
          AND end_date >= ? AND start_date <= ?
        ORDER BY s
    """).use { ps ->
        ps.setDate(1, Date.valueOf(want.from)); ps.setDate(2, Date.valueOf(want.to))
        ps.setString(3, s.source); ps.setString(4, s.symbol); ps.setString(5, s.schema)
        ps.setDate(6, Date.valueOf(want.from)); ps.setDate(7, Date.valueOf(want.to))
        ps.executeQuery().use { rs ->
            buildList { while (rs.next()) add(DateRange(rs.getDate(1).toLocalDate(), rs.getDate(2).toLocalDate())) }
        }
    }

/**
 * Is [want] entirely held? Because ranges are merged on write, one stored row must
 * span it — no interval reasoning needed.
 */
fun Connection.isCovered(s: Slice, want: DateRange): Boolean =
    prepareStatement("""
        SELECT EXISTS (
          SELECT 1 FROM coverage
          WHERE source = ? AND symbol = ? AND schema = ?
            AND start_date <= ? AND end_date >= ?
        )""").use { ps ->
        ps.setString(1, s.source); ps.setString(2, s.symbol); ps.setString(3, s.schema)
        ps.setDate(4, Date.valueOf(want.from)); ps.setDate(5, Date.valueOf(want.to))
        ps.executeQuery().use { it.next(); it.getBoolean(1) }
    }

/** The sub-ranges of [want] not yet held — exactly what still has to be fetched. */
fun Connection.missingRanges(s: Slice, want: DateRange): List<DateRange> {
    val held = clippedRanges(s, want)
    if (held.isEmpty()) return listOf(want)

    val gaps = mutableListOf<DateRange>()
    var cursor = want.from
    for (r in held) {
        if (r.from.isAfter(cursor)) gaps += DateRange(cursor, r.from.minusDays(1))
        if (r.to.isAfter(cursor.minusDays(1))) cursor = maxOf(cursor, r.to.plusDays(1))
    }
    if (!cursor.isAfter(want.to)) gaps += DateRange(cursor, want.to)
    return gaps
}

/**
 * Record a fetched range, merging into anything it overlaps or abuts. Merging on
 * write is what keeps [isCovered] a single EXISTS instead of a recursive CTE.
 *
 * [rows] = 0 is a real answer: "asked, nothing there." Recording it stops a
 * delisted or pre-IPO symbol being re-requested on every universe scan.
 */
fun Connection.recordCoverage(s: Slice, got: DateRange, rows: Long) {
    val touching = prepareStatement("""
        SELECT start_date, end_date, rows FROM coverage
        WHERE source = ? AND symbol = ? AND schema = ?
          AND end_date >= ? AND start_date <= ?
    """).use { ps ->
        ps.setString(1, s.source); ps.setString(2, s.symbol); ps.setString(3, s.schema)
        ps.setDate(4, Date.valueOf(got.from.minusDays(1)))   // -1/+1 so abutting ranges merge
        ps.setDate(5, Date.valueOf(got.to.plusDays(1)))
        ps.executeQuery().use { rs ->
            buildList<Triple<LocalDate, LocalDate, Long>> {
                while (rs.next()) add(Triple(rs.getDate(1).toLocalDate(), rs.getDate(2).toLocalDate(), rs.getLong(3)))
            }
        }
    }

    val from = (touching.map { it.first } + got.from).min()
    val to = (touching.map { it.second } + got.to).max()
    val total = touching.sumOf { it.third } + rows

    prepareStatement("""
        DELETE FROM coverage
        WHERE source = ? AND symbol = ? AND schema = ? AND end_date >= ? AND start_date <= ?
    """).use { ps ->
        ps.setString(1, s.source); ps.setString(2, s.symbol); ps.setString(3, s.schema)
        ps.setDate(4, Date.valueOf(got.from.minusDays(1))); ps.setDate(5, Date.valueOf(got.to.plusDays(1)))
        ps.executeUpdate()
    }
    prepareStatement(
        "INSERT INTO coverage (source, symbol, schema, start_date, end_date, rows) VALUES (?,?,?,?,?,?)"
    ).use { ps ->
        ps.setString(1, s.source); ps.setString(2, s.symbol); ps.setString(3, s.schema)
        ps.setDate(4, Date.valueOf(from)); ps.setDate(5, Date.valueOf(to)); ps.setLong(6, total)
        ps.executeUpdate()
    }
}
