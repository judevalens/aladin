/**
 * Vendor payload decoding, separated from transport.
 *
 * These are pure: text in, domain objects out. Every bug in this integration has been
 * a decoding bug — nanoseconds read as ISO, a `symbol` column that wasn't there,
 * validity intervals clipped to the query window — and none of them were reachable by
 * a test while the parsing lived inside a method that also made an HTTP call.
 */

import kotlinx.serialization.json.*
import java.time.LocalDate
import java.time.OffsetDateTime

/** Stands for "at or before the data begins" — never a claim about a listing date. */
val DATASET_FLOOR: LocalDate = LocalDate.parse("1900-01-01")

object OhlcvCsv {
    /**
     * Decode an ohlcv CSV response. Requires `pretty_px`/`pretty_ts` (decimal prices,
     * ISO timestamps) and `map_symbols` (the `symbol` column) — without the last one
     * rows cannot be attributed and a batched request is silently unusable.
     *
     * Rows for symbols outside [instruments] are dropped; row order is not stable, so
     * nothing may depend on it.
     */
    fun parse(csv: String, instruments: Map<String, Long>): List<BarRow> {
        val lines = csv.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size <= 1) return emptyList()

        val h = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        for (c in listOf("ts_event", "symbol", "close")) {
            require(c in h) { "no `$c` column — need map_symbols=true and pretty flags. Got: ${lines.first()}" }
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

    /** Rows the server says exist, so silent truncation cannot pass for missing data. */
    fun assertComplete(csv: String, expected: Long) {
        val got = csv.lineSequence().filter { it.isNotBlank() }.count() - 1
        check(expected < 0 || got.toLong() == expected) {
            "truncated: parsed $got rows, server reports $expected"
        }
    }
}

object SymbologyJson {
    /**
     * Decode `symbology.resolve`. `result[symbol]` is a list of `{d0, d1, s}` intervals;
     * more than one means the ticker denoted different instruments over time.
     *
     * **Intervals are clipped to the requested window**, so a boundary is only a real
     * listing or delisting when it falls strictly inside [from]..[to]. Otherwise validity
     * is left open rather than inventing a date the vendor never asserted.
     */
    fun parse(json: String, symbol: String, from: LocalDate, to: LocalDate): List<Instrument> {
        val body = Json.parseToJsonElement(json).jsonObject
        val notFound = body["not_found"]?.jsonArray.orEmpty().map { it.jsonPrimitive.content }
        if (symbol in notFound) return emptyList()

        return body["result"]?.jsonObject?.get(symbol)?.jsonArray.orEmpty().mapNotNull { iv ->
            val o = iv.jsonObject
            val id = o["s"]?.jsonPrimitive?.content?.toLongOrNull() ?: return@mapNotNull null
            val d0 = LocalDate.parse(o["d0"]!!.jsonPrimitive.content)
            val d1 = LocalDate.parse(o["d1"]!!.jsonPrimitive.content)
            Instrument(
                id = id, symbol = symbol,
                validFrom = if (d0 > from) d0 else DATASET_FLOOR,
                validTo = if (d1 < to) d1 else null,
            )
        }
    }
}
