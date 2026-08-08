package aladin.store

import aladin.Schema
import java.sql.Connection
import java.time.LocalDate

/**
 * A break in an instrument's history that suggests it is not one instrument.
 *
 * [gapDays] of silence and a [ratio] jump across it. Either alone is ordinary — a halt
 * or suspension explains a gap, a split or a SPAC completion explains a jump — but
 * together they are what a **recycled ticker** looks like from the data.
 */
data class IdentityBreak(
    val instrumentId: Long,
    val symbol: String,
    val lastBefore: LocalDate,
    val firstAfter: LocalDate,
    val gapDays: Long,
    val closeBefore: Double,
    val closeAfter: Double,
) {
    val ratio: Double get() = closeAfter / closeBefore

    override fun toString() =
        "$symbol (id $instrumentId): silent $gapDays days from $lastBefore to $firstAfter, " +
            "then ${"%.2f".format(closeBefore)} -> ${"%.2f".format(closeAfter)} (${"%.1f".format(ratio)}x)"
}

/**
 * Instruments whose stored history looks like two different companies.
 *
 * The store keys on `instrument_id` precisely so a recycled ticker cannot merge two
 * companies into one price series — but that guarantee is only as good as the vendor's
 * identity layer, and vendors get it wrong. Databento maps SPCX to a single
 * instrument_id from 2020 to now, across the ticker moving from Tuttle Capital's SPAC
 * ETF to SpaceX's IPO: a $23 fund and a $150 rocket company in one series, with a 7x
 * "return" no holder ever saw.
 *
 * Nothing upstream flags that. This does, from the shape of the data alone.
 *
 * Not a resolver — a smoke alarm. A hit means go and look, not that the data is wrong.
 * The fix when one is real is an explicit [BarStore.register] splitting the instrument
 * into two validity windows, which the as-of resolution then honours.
 */
fun Connection.identityBreaks(
    source: String,
    schema: Schema = Schema.OHLCV_1D,
    minGapDays: Long = 90,
    minRatio: Double = 2.0,
): List<IdentityBreak> =
    prepareStatement(
        """
        WITH bars AS (
            SELECT o.instrument_id, o.ts::DATE AS d, o.close,
                   lag(o.ts::DATE) OVER (PARTITION BY o.instrument_id ORDER BY o.ts) AS prev_d,
                   lag(o.close)    OVER (PARTITION BY o.instrument_id ORDER BY o.ts) AS prev_close
            FROM ohlcv o
            WHERE o.source = ? AND o.schema = ? AND o.close IS NOT NULL
        )
        SELECT b.instrument_id, i.symbol, b.prev_d, b.d, (b.d - b.prev_d) AS gap, b.prev_close, b.close
        FROM bars b
        LEFT JOIN instruments i ON i.instrument_id = b.instrument_id
             AND b.d >= i.valid_from AND (i.valid_to IS NULL OR b.d <= i.valid_to)
        WHERE b.prev_d IS NOT NULL
          AND (b.d - b.prev_d) >= ?
          AND b.prev_close > 0
          AND (b.close / b.prev_close >= ? OR b.prev_close / b.close >= ?)
        ORDER BY gap DESC
        """
    ).use { ps ->
        ps.setString(1, source)
        ps.setString(2, schema.wire)
        ps.setLong(3, minGapDays)
        ps.setDouble(4, minRatio)
        ps.setDouble(5, minRatio)
        ps.executeQuery().use { rs ->
            buildList {
                while (rs.next()) add(
                    IdentityBreak(
                        instrumentId = rs.getLong(1),
                        symbol = rs.getString(2) ?: "?",
                        lastBefore = rs.getDate(3).toLocalDate(),
                        firstAfter = rs.getDate(4).toLocalDate(),
                        gapDays = rs.getLong(5),
                        closeBefore = rs.getDouble(6),
                        closeAfter = rs.getDouble(7),
                    )
                )
            }
        }
    }
