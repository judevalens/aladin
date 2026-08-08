/**
 * Instrument identity from Databento's symbology.
 *
 * Response shape, verified against the live endpoint (the client source returns raw
 * json and the published reference would not load):
 *
 *     {"result": {"AAPL": [{"d0":"2024-08-01","d1":"2026-08-01","s":"38"}]},
 *      "not_found": ["NOPE"], "partial": [], "status": 0}
 *
 * `result[symbol]` is a LIST of intervals — more than one means the ticker denoted
 * different instruments over time, which is the whole reason identity is not the symbol.
 *
 * **Intervals are clipped to the requested window.** Asking about AAPL over
 * 2024-08-01..2024-08-05 returns exactly those dates, which says nothing about when it
 * listed. So queries here span the dataset's entire range, and a boundary is only
 * treated as real when it falls strictly inside that range — otherwise validity is
 * left open rather than inventing a listing or delisting date.
 */

import kotlinx.serialization.json.*
import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.LocalDate
import java.util.Base64
import kotlin.text.Charsets.UTF_8

class DatabentoSymbology(
    private val apiKey: String = Env.require("DATABENTO_API_KEY"),
    private val dataset: String = Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY",
    private val http: HttpClient = HttpClient.newHttpClient(),
) : SymbologySource {

    override val source = "databento:$dataset"

    /** The dataset's own bounds — asking outside them teaches nothing. */
    private val bounds: Pair<LocalDate, LocalDate> by lazy {
        val o = Json.parseToJsonElement(call("metadata.get_dataset_range", mapOf("dataset" to dataset), get = true)).jsonObject
        fun date(k: String) = LocalDate.parse(o[k]!!.jsonPrimitive.content.substring(0, 10))
        date("start") to date("end")
    }

    override fun history(symbol: String): List<Instrument> {
        val (from, to) = bounds
        val json = call("symbology.resolve", mapOf(
            "dataset" to dataset, "symbols" to symbol,
            "stype_in" to "raw_symbol", "stype_out" to "instrument_id",
            "start_date" to from.toString(), "end_date" to to.toString(),
        ))

        return SymbologyJson.parse(json, symbol, from, to)
    }

    private fun call(endpoint: String, params: Map<String, String>, get: Boolean = false): String {
        val form = params.entries.joinToString("&") { (k, v) ->
            "${URLEncoder.encode(k, UTF_8)}=${URLEncoder.encode(v, UTF_8)}"
        }
        val auth = Base64.getEncoder().encodeToString("$apiKey:".toByteArray())
        val b = HttpRequest.newBuilder(
            URI.create("https://hist.databento.com/v0/$endpoint" + if (get) "?$form" else "")
        ).header("Authorization", "Basic $auth")
        val req = if (get) b.GET().build()
        else b.header("Content-Type", "application/x-www-form-urlencoded")
            .POST(HttpRequest.BodyPublishers.ofString(form)).build()
        val res = http.send(req, HttpResponse.BodyHandlers.ofString())
        check(res.statusCode() == 200) { "databento $endpoint -> ${res.statusCode()}: ${res.body().take(300)}" }
        return res.body()
    }

}
