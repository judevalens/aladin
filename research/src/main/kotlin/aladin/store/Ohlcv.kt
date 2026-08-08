package aladin.store

import aladin.DateRange
import aladin.Schema
import aladin.Slice
import aladin.vendor.BarFetcher
import java.sql.Connection
import java.sql.Timestamp
import java.time.DayOfWeek
import java.time.LocalDate

/** Columns a caller may ask for as the matrix value. Anything else is rejected. */
val PRICE_FIELDS: Set<String> = setOf("open", "high", "low", "close", "volume")

fun Connection.createOhlcvTable() = createStatement().use {
    it.execute(
        """
        CREATE TABLE IF NOT EXISTS ohlcv (
          source        VARCHAR   NOT NULL,          -- data SCOPE, not vendor
          instrument_id BIGINT    NOT NULL,
          schema        VARCHAR   NOT NULL,
          ts            TIMESTAMP NOT NULL,          -- TIMESTAMP so intraday fits too
          open DOUBLE, high DOUBLE, low DOUBLE, close DOUBLE,   -- nullable: a halt is not 0.0
          volume   BIGINT,
          adjusted BOOLEAN NOT NULL,                 -- raw vs vendor-adjusted; never assume
          -- Coverage should prevent duplicates, but it must not be the only thing that
          -- does: doubled bars produce plausible wrong numbers rather than errors.
          PRIMARY KEY (source, instrument_id, schema, ts)
        )
        """
    )
}

/**
 * The last session that is definitely complete.
 *
 * Today's bar is partial until the close, so recording it as covered would freeze an
 * incomplete day permanently. Weekend-aware only — a real trading calendar (holidays,
 * half days) is still owed.
 */
fun lastSettledSession(today: LocalDate = LocalDate.now()): LocalDate {
    var d = today.minusDays(1)
    while (d.dayOfWeek == DayOfWeek.SATURDAY || d.dayOfWeek == DayOfWeek.SUNDAY) d = d.minusDays(1)
    return d
}

/**
 * Guards the whole read-through cycle — gap computation, fetch, bar insert, coverage
 * write.
 *
 * Locking only the fetch is not enough: DuckDB's MVCC rejects the second of two
 * concurrent DELETE+INSERTs on one coverage row, and by then the fetch has been paid
 * for. A single global lock is right here — DuckDB is single-writer and the engine is
 * one JVM, so finer granularity would buy nothing.
 */
private val ensureLock = Any()

/**
 * Ensure [range] is held for every instrument, fetching only the gaps.
 *
 * Gaps are computed per instrument then grouped, so instruments sharing a gap go out in
 * one vendor request. Returns bars newly fetched; 0 means it was already a full hit.
 */
fun Connection.ensureBars(
    fetcher: BarFetcher,
    instruments: Map<String, Long>,
    schema: Schema,
    range: DateRange,
): Long = synchronized(ensureLock) {
    if (instruments.isEmpty()) return@synchronized 0

    val settled = lastSettledSession()
    if (range.from.isAfter(settled)) return@synchronized 0
    val want = DateRange(range.from, minOf(range.to, settled))

    val perGap = instruments.entries.flatMap { (sym, id) ->
        missingRanges(Slice(fetcher.source, id, schema), want).map { gap -> gap to (sym to id) }
    }
    if (perGap.isEmpty()) return@synchronized 0

    var fetched = 0L
    for ((gap, group) in perGap.groupBy({ it.first }, { it.second })) {
        val batch = group.toMap()
        val rows = fetcher.fetch(batch, schema, gap)

        // Bars and coverage must land together. A crash between them would leave the
        // ledger claiming data the store lacks — the one unrecoverable failure here,
        // since nothing would ever re-fetch it.
        val restore = autoCommit
        autoCommit = false
        try {
            prepareStatement("INSERT INTO ohlcv VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING")
                .use { ps ->
                    for (b in rows) {
                        ps.setString(1, fetcher.source)
                        ps.setLong(2, b.instrumentId)
                        ps.setString(3, schema.wire)
                        ps.setTimestamp(4, Timestamp.valueOf(b.ts))
                        ps.setObject(5, b.open); ps.setObject(6, b.high)
                        ps.setObject(7, b.low); ps.setObject(8, b.close); ps.setObject(9, b.volume)
                        ps.setBoolean(10, fetcher.adjusted)
                        ps.addBatch()
                    }
                    ps.executeBatch()
                }
            batch.values.forEach { recordCoverage(Slice(fetcher.source, it, schema), gap) }
            commit()
        } catch (e: Exception) {
            rollback()
            throw e
        } finally {
            autoCommit = restore
        }
        fetched += rows.size
    }
    return@synchronized fetched
}
