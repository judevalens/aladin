package aladin.vendor

import aladin.BarRow
import aladin.Instrument
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import java.time.LocalDate
import java.time.OffsetDateTime

/**
 * Databento payload decoding, deliberately separate from transport.
 *
 * Every bug in this integration has been a decoding bug — nanoseconds read as ISO, a
 * `symbol` column that was not there, validity intervals taken at face value — and none
 * were reachable by a test while decoding lived inside a method that also made an HTTP
 * call. These are pure: text in, domain objects out.
 */

/** Stands for "at or before the data begins". Never a claim about a listing date. */
val DATASET_FLOOR: LocalDate = LocalDate.parse("1900-01-01")

object OhlcvCsv {
    /**
     * Decode an ohlcv CSV response.
     *
     * Requires `pretty_px` and `pretty_ts` (decimal prices, ISO timestamps) and
     * `map_symbols` (the `symbol` column). Without the last, rows cannot be attributed
     * and a batched request is silently unusable — so its absence is an error, not a
     * fallback.
     *
     * Rows for symbols outside [instruments] are dropped. Row order is not stable across
     * requests, so nothing may depend on it.
     */
    fun parse(csv: String, instruments: Map<String, Long>): List<BarRow> {
        val lines = csv.lineSequence().filter(String::isNotBlank).toList()
        if (lines.size <= 1) return emptyList()

        val h = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        for (c in listOf("ts_event", "symbol", "close")) {
            require(c in h) {
                "no `$c` column — the request needs map_symbols=true and the pretty flags. " +
                    "Got: ${lines.first()}"
            }
        }
        return lines.drop(1).mapNotNull { line ->
            val p = line.split(",")
            val id = instruments[p[h.getValue("symbol")]] ?: return@mapNotNull null
            fun num(c: String) = h[c]?.let { p[it].toDoubleOrNull() }
            BarRow(
                ts = OffsetDateTime.parse(p[h.getValue("ts_event")]).toLocalDateTime(),
                instrumentId = id,
                open = num("open"), high = num("high"), low = num("low"), close = num("close"),
                volume = h["volume"]?.let { p[it].toLongOrNull() },
            )
        }
    }

    /**
     * Check the row count against what the server says exists.
     *
     * Pagination on the streaming endpoint is undocumented, and silent truncation is the
     * one failure that looks exactly like missing data. A negative [expected] means the
     * count was unavailable.
     */
    fun assertComplete(csv: String, expected: Long) {
        val got = csv.lineSequence().filter(String::isNotBlank).count() - 1
        check(expected < 0 || got.toLong() == expected) {
            "truncated: parsed $got rows, server reports $expected — refusing to store a partial result"
        }
    }
}

object SymbologyJson {
    /**
     * Decode `symbology.resolve`.
     *
     * `result[symbol]` is a list of `{d0, d1, s}` intervals; more than one means the
     * ticker denoted different instruments over time, which is why identity is not the
     * symbol.
     *
     * **Intervals are clipped to the requested window**, so a boundary is a real listing
     * or delisting only when it falls strictly inside [from]..[to]. Otherwise validity is
     * left open rather than inventing a date the vendor never asserted.
     */
    fun parse(json: String, symbol: String, from: LocalDate, to: LocalDate): List<Instrument> {
        val body = Json.parseToJsonElement(json).jsonObject
        val notFound = body["not_found"]?.jsonArray.orEmpty().map { it.jsonPrimitive.content }
        if (symbol in notFound) return emptyList()

        return body["result"]?.jsonObject?.get(symbol)?.jsonArray.orEmpty().mapNotNull { interval ->
            val o = interval.jsonObject
            val id = o["s"]?.jsonPrimitive?.content?.toLongOrNull() ?: return@mapNotNull null
            val d0 = LocalDate.parse(o["d0"]!!.jsonPrimitive.content)
            val d1 = LocalDate.parse(o["d1"]!!.jsonPrimitive.content)
            Instrument(
                id = id,
                symbol = symbol,
                validFrom = if (d0 > from) d0 else DATASET_FLOOR,
                validTo = if (d1 < to) d1 else null,
            )
        }
    }
}
