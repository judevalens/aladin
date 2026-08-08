/**
 * The coverage ledger: which (instrument, schema, date range) slices we already hold.
 *
 * This cannot be derived from the bars table. A missing row is ambiguous — weekend,
 * holiday, trading halt, pre-IPO, post-delisting, or genuinely not fetched. Only a
 * record of what was *requested* distinguishes them.
 *
 * Ranges are stored merged and inclusive on both ends, so the common question —
 * "do I have all of this?" — is a single EXISTS. Gap-finding for partial coverage is
 * interval arithmetic in Kotlin, because a slice's ledger is a handful of rows and the
 * logic is far clearer in code than in recursive SQL.
 */

import java.sql.Connection
import java.sql.Date
import java.time.LocalDate

data class DateRange(val from: LocalDate, val to: LocalDate) {
    init { require(!from.isAfter(to)) { "empty range: $from..$to" } }
    val days get() = java.time.temporal.ChronoUnit.DAYS.between(from, to) + 1
    override fun toString() = "$from..$to"
}

/** Keyed on instrument_id, never symbol — see Instruments.kt. */
data class Slice(val source: String, val instrumentId: Long, val schema: Schema)

fun Connection.createCoverageTable() = createStatement().use {
    it.execute("""
        CREATE TABLE IF NOT EXISTS coverage (
          source        VARCHAR NOT NULL,      -- 'databento' | 'alpaca'
          instrument_id BIGINT  NOT NULL,
          schema        VARCHAR NOT NULL,      -- 'ohlcv-1d' | 'ohlcv-1m'
          start_date    DATE    NOT NULL,      -- inclusive
          end_date      DATE    NOT NULL,      -- inclusive
          rows          BIGINT  NOT NULL,      -- 0 is meaningful: checked, legitimately empty
          fetched_at    TIMESTAMP NOT NULL DEFAULT now(),
          PRIMARY KEY (source, instrument_id, schema, start_date)
        )""")
}

/** Every stored range for this slice that touches [want], clipped to it, ordered. */
private fun Connection.clippedRanges(s: Slice, want: DateRange): List<DateRange> =
    prepareStatement("""
        SELECT greatest(start_date, ?) AS s, least(end_date, ?) AS e
        FROM coverage
        WHERE source = ? AND instrument_id = ? AND schema = ?
          AND end_date >= ? AND start_date <= ?
        ORDER BY s
    """).use { ps ->
        ps.setDate(1, Date.valueOf(want.from)); ps.setDate(2, Date.valueOf(want.to))
        ps.setString(3, s.source); ps.setLong(4, s.instrumentId); ps.setString(5, s.schema.wire)
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
          WHERE source = ? AND instrument_id = ? AND schema = ?
            AND start_date <= ? AND end_date >= ?
        )""").use { ps ->
        ps.setString(1, s.source); ps.setLong(2, s.instrumentId); ps.setString(3, s.schema.wire)
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
        if (!r.to.isBefore(cursor)) cursor = maxOf(cursor, r.to.plusDays(1))
    }
    if (!cursor.isAfter(want.to)) gaps += DateRange(cursor, want.to)
    return gaps
}

/**
 * Record a fetched range, merging into anything it overlaps or abuts. Merging on write
 * is what keeps [isCovered] a single EXISTS instead of a recursive CTE.
 *
 * The row count is **recounted from the store** rather than summed across the merged
 * ranges — summing double-counts any overlap, and a count that lies is worse than none.
 *
 * A range that yielded nothing is still recorded. "Asked, nothing there" is a real
 * answer, and storing it stops a delisted or pre-IPO instrument being re-requested on
 * every universe scan.
 */
fun Connection.recordCoverage(s: Slice, got: DateRange) {
    val touching = prepareStatement("""
        SELECT min(start_date), max(end_date) FROM coverage
        WHERE source = ? AND instrument_id = ? AND schema = ?
          AND end_date >= ? AND start_date <= ?
    """).use { ps ->
        ps.setString(1, s.source); ps.setLong(2, s.instrumentId); ps.setString(3, s.schema.wire)
        ps.setDate(4, Date.valueOf(got.from.minusDays(1)))   // -1/+1 so abutting ranges merge
        ps.setDate(5, Date.valueOf(got.to.plusDays(1)))
        ps.executeQuery().use { rs ->
            rs.next()
            rs.getDate(1)?.toLocalDate() to rs.getDate(2)?.toLocalDate()
        }
    }
    val from = minOf(got.from, touching.first ?: got.from)
    val to = maxOf(got.to, touching.second ?: got.to)

    val actual = prepareStatement("""
        SELECT count(*) FROM ohlcv
        WHERE source = ? AND instrument_id = ? AND schema = ?
          AND ts >= ? AND ts < ?
    """).use { ps ->
        ps.setString(1, s.source); ps.setLong(2, s.instrumentId); ps.setString(3, s.schema.wire)
        ps.setTimestamp(4, java.sql.Timestamp.valueOf(from.atStartOfDay()))
        ps.setTimestamp(5, java.sql.Timestamp.valueOf(to.plusDays(1).atStartOfDay()))
        ps.executeQuery().use { it.next(); it.getLong(1) }
    }

    prepareStatement("""
        DELETE FROM coverage
        WHERE source = ? AND instrument_id = ? AND schema = ? AND end_date >= ? AND start_date <= ?
    """).use { ps ->
        ps.setString(1, s.source); ps.setLong(2, s.instrumentId); ps.setString(3, s.schema.wire)
        ps.setDate(4, Date.valueOf(got.from.minusDays(1))); ps.setDate(5, Date.valueOf(got.to.plusDays(1)))
        ps.executeUpdate()
    }
    prepareStatement(
        "INSERT INTO coverage (source, instrument_id, schema, start_date, end_date, rows) VALUES (?,?,?,?,?,?)"
    ).use { ps ->
        ps.setString(1, s.source); ps.setLong(2, s.instrumentId); ps.setString(3, s.schema.wire)
        ps.setDate(4, Date.valueOf(from)); ps.setDate(5, Date.valueOf(to)); ps.setLong(6, actual)
        ps.executeUpdate()
    }
}
