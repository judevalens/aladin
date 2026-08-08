package aladin.vendor

import aladin.BarRow
import aladin.DateRange
import aladin.Env
import aladin.Instrument
import aladin.Schema
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.longOrNull
import java.io.File
import java.time.Duration
import java.time.LocalDate

/**
 * Databento clients.
 *
 * Verified against real responses and the official Python client, which is the
 * authoritative contract — the published reference would not load:
 *
 *  - `timeseries.get_range` is **POST, form-encoded**, max **2,000 symbols** per
 *    request. There is no pagination; `limit` is opt-in.
 *  - `pretty_px=true` and `pretty_ts=true` make the server return decimal prices and
 *    ISO timestamps. Without them prices are int64 scaled by 1e-9 and timestamps are
 *    nanoseconds, both decoded by hand — and a wrong scale silently corrupts every price.
 *  - `stype_in=raw_symbol` or tickers are read as vendor instrument ids;
 *    `map_symbols=true` or there is no `symbol` column.
 *  - `end` is **exclusive**; row order is **not** stable.
 *
 * Dataset choice is not cosmetic. `EQUS.SUMMARY` is the consolidated tape — AAPL on
 * 2024-08-01 is 62,500,996 shares, matching Alpaca to the share — and carries
 * ohlcv-1d/definition/statistics only, from 2024-07-01. `XNAS.ITCH` reports 21,277,576
 * for that bar: correct for Nasdaq, wrong for a backtest.
 */
private const val MAX_SYMBOLS_PER_REQUEST = 2_000

private fun defaultDataset() = Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY"

/**
 * What a dataset can serve. Looked up once per client and cached — it changes only as
 * the vendor ingests more, and a stale upper bound costs at most a wasted day.
 */
internal fun datasetRange(http: VendorHttp, dataset: String): DateRange {
    val o = Json.parseToJsonElement(
        http.get("metadata.get_dataset_range", mapOf("dataset" to dataset))
    ).jsonObject
    fun date(k: String) = LocalDate.parse(o[k]!!.jsonPrimitive.content.substring(0, 10))
    return DateRange(date("start"), date("end"))
}

/** Streaming client — suits read-through gaps, where the answer is wanted now. */
class DatabentoFetcher(
    private val dataset: String = defaultDataset(),
    private val http: VendorHttp = VendorHttp.databento(),
) : PricedFetcher, AutoCloseable {

    override fun close() = http.close()

    override val source = "databento:$dataset"
    override val adjusted = false          // Databento serves raw venue data
    override val availability: DateRange by lazy { datasetRange(http, dataset) }

    override fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange): Double =
        http.post("metadata.get_cost", params(symbols, schema, range)).trim().toDoubleOrNull() ?: 0.0

    /**
     * Memoised on the last query, because a fetch asks twice with identical arguments:
     * once through the budget gate to describe the request, once inside [fetch] to guard
     * against truncation. Each is a round trip, and they are always back to back.
     */
    private var lastCount: Pair<String, Long>? = null

    override fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange): Long {
        val key = "${symbols.sorted()}|${schema.wire}|$range"
        lastCount?.let { (k, v) -> if (k == key) return v }
        val n = http.post("metadata.get_record_count", params(symbols, schema, range))
            .trim().toLongOrNull() ?: -1
        lastCount = key to n
        return n
    }

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> =
        instruments.keys.chunked(MAX_SYMBOLS_PER_REQUEST).flatMap { chunk ->
            val expected = runCatching { recordCount(chunk, schema, range) }.getOrDefault(-1L)
            val csv = http.post(
                "timeseries.get_range",
                params(chunk, schema, range) + mapOf(
                    "encoding" to "csv", "map_symbols" to "true",
                    "pretty_px" to "true", "pretty_ts" to "true",
                ),
            )
            OhlcvCsv.assertComplete(csv, expected)
            OhlcvCsv.parse(csv, instruments)
        }

    private fun params(symbols: Collection<String>, schema: Schema, range: DateRange) = mapOf(
        "dataset" to dataset,
        "symbols" to symbols.joinToString(","),
        "schema" to schema.wire,
        "stype_in" to "raw_symbol",
        "start" to range.from.toString(),
        "end" to range.to.plusDays(1).toString(),      // end is exclusive
    )
}

data class BatchJob(val id: String, val state: String, val recordCount: Long?, val costUsd: Double?)

/**
 * Batch client — submit, poll, download. Suits bulk, where waiting is acceptable.
 *
 * Same [BarFetcher] interface as the streaming client, so the store is indifferent to
 * which it is given.
 */
class DatabentoBatchFetcher(
    private val dataset: String = defaultDataset(),
    private val workDir: File = File("data/batch"),
    private val pollEvery: Duration = Duration.ofSeconds(5),
    private val timeout: Duration = Duration.ofMinutes(30),
    private val http: VendorHttp = VendorHttp.databento(),
) : BarFetcher, AutoCloseable {

    override fun close() = http.close()

    override val source = "databento:$dataset"
    override val adjusted = false
    override val availability: DateRange by lazy { datasetRange(http, dataset) }

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        val job = submit(instruments.keys, schema, range)
        val done = awaitDone(job.id)
        check(done.state == "done") { "job ${job.id} ended in state '${done.state}'" }

        val rows = download(job.id)
            .filter { it.name.endsWith(".csv") }
            .flatMap { OhlcvCsv.parse(it.readText(), instruments) }

        done.recordCount?.let {
            check(rows.size.toLong() == it) { "job ${job.id}: parsed ${rows.size} rows, job reports $it" }
        }
        return rows
    }

    fun submit(symbols: Collection<String>, schema: Schema, range: DateRange): BatchJob =
        Json.parseToJsonElement(
            http.post(
                "batch.submit_job",
                mapOf(
                    "dataset" to dataset,
                    "symbols" to symbols.joinToString(","),
                    "schema" to schema.wire,
                    "stype_in" to "raw_symbol",
                    "start" to range.from.toString(),
                    "end" to range.to.plusDays(1).toString(),   // exclusive
                    // csv + none on purpose: DBN needs a decoder and zstd a dependency,
                    // and neither earns its place until volume justifies it
                    "encoding" to "csv",
                    "compression" to "none",
                    "map_symbols" to "true",
                    "pretty_px" to "true",
                    "pretty_ts" to "true",
                    "delivery" to "download",
                ),
            )
        ).jsonObject.toJob()

    fun awaitDone(jobId: String): BatchJob {
        val deadline = System.nanoTime() + timeout.toNanos()
        while (true) {
            val job = Json.parseToJsonElement(http.get("batch.list_jobs")).jsonArray
                .map { it.jsonObject.toJob() }
                .firstOrNull { it.id == jobId }
                ?: error("job $jobId vanished from list_jobs")

            if (job.state == "done" || job.state == "expired") return job
            check(System.nanoTime() < deadline) { "job $jobId still '${job.state}' after $timeout" }
            Thread.sleep(pollEvery.toMillis())
        }
    }

    /** Download every file the job produced into workDir/<jobId>/, skipping any already there. */
    fun download(jobId: String): List<File> {
        val dir = File(workDir, jobId).apply { mkdirs() }
        return Json.parseToJsonElement(http.get("batch.list_files", mapOf("job_id" to jobId)))
            .jsonArray.mapNotNull { f ->
                val o = f.jsonObject
                val name = o["filename"]?.jsonPrimitive?.content ?: return@mapNotNull null
                val url = o["urls"]?.jsonObject?.get("https")?.jsonPrimitive?.content ?: return@mapNotNull null
                File(dir, name).also { if (!it.exists()) http.download(url, it) }
            }
    }

    private fun kotlinx.serialization.json.JsonObject.toJob() = BatchJob(
        id = this["id"]!!.jsonPrimitive.content,
        state = this["state"]!!.jsonPrimitive.content,
        recordCount = this["record_count"]?.jsonPrimitive?.longOrNull,
        costUsd = this["cost_usd"]?.jsonPrimitive?.doubleOrNull,
    )
}

/**
 * Instrument identity from `symbology.resolve`.
 *
 * Queries span the dataset's whole range, because the response clips intervals to the
 * window asked about — a three-day query returns three days and says nothing about when
 * the instrument listed.
 */
class DatabentoSymbology(
    private val dataset: String = defaultDataset(),
    private val http: VendorHttp = VendorHttp.databento(),
) : SymbologySource, AutoCloseable {

    override fun close() = http.close()

    override val source = "databento:$dataset"

    /** The dataset's own bounds — asking outside them teaches nothing. */
    private val bounds: DateRange by lazy { datasetRange(http, dataset) }

    override fun history(symbol: String): List<Instrument> {
        val (from, to) = bounds
        val json = http.post(
            "symbology.resolve",
            mapOf(
                "dataset" to dataset, "symbols" to symbol,
                "stype_in" to "raw_symbol", "stype_out" to "instrument_id",
                "start_date" to from.toString(), "end_date" to to.toString(),
            ),
        )
        return SymbologyJson.parse(json, symbol, from, to)
    }
}
