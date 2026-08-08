/**
 * Databento's batch path: submit a job, poll until it's done, download the files.
 *
 * Same [BarFetcher] interface as the streaming client, so the store is indifferent —
 * `ensureBars` works with either. The difference is transport: streaming returns the
 * data in the response, batch queues work server-side and delivers files. Use this
 * when a gap is large enough that a synchronous response is inappropriate;
 * `metadata.get_record_count` tells you which you're facing before committing.
 *
 * Shaped from the official Python client, which is the authoritative contract:
 *   POST batch.submit_job  -> job object with `id` and `state`
 *   GET  batch.list_jobs   -> states are queued -> processing -> done
 *   GET  batch.list_files  -> filenames, sizes, hashes, https urls
 *
 * `encoding=csv` and `compression=none` on purpose: DBN would need a decoder and
 * zstd a dependency, and neither earns its place until volume justifies it.
 */

import kotlinx.serialization.json.*
import java.io.File
import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration
import java.time.LocalDateTime
import java.time.OffsetDateTime
import java.util.Base64
import kotlin.text.Charsets.UTF_8

data class BatchJob(val id: String, val state: String, val recordCount: Long?, val costUsd: Double?)

class DatabentoBatchFetcher(
    private val apiKey: String = Env.require("DATABENTO_API_KEY"),
    private val dataset: String = Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY",
    private val workDir: File = File("data/batch"),
    private val pollEvery: Duration = Duration.ofSeconds(5),
    private val timeout: Duration = Duration.ofMinutes(30),
    private val http: HttpClient = HttpClient.newHttpClient(),
) : BarFetcher {
    override val source = "databento:$dataset"
    override val adjusted = false

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        val job = submit(instruments.keys, schema, range)
        val done = awaitDone(job.id)
        check(done.state == "done") { "job ${job.id} ended in state '${done.state}'" }

        val files = download(job.id)
        val rows = files.filter { it.name.endsWith(".csv") }.flatMap { parse(it, instruments) }
        done.recordCount?.let {
            check(rows.size.toLong() == it) { "job ${job.id}: parsed ${rows.size} rows, job reports $it" }
        }
        return rows
    }

    fun submit(symbols: Collection<String>, schema: Schema, range: DateRange): BatchJob {
        val body = form(mapOf(
            "dataset" to dataset,
            "symbols" to symbols.joinToString(","),
            "schema" to schema.wire,
            "stype_in" to "raw_symbol",
            "start" to range.from.toString(),
            "end" to range.to.plusDays(1).toString(),        // end is exclusive
            "encoding" to "csv",
            "compression" to "none",
            "map_symbols" to "true",
            "pretty_px" to "true",
            "pretty_ts" to "true",
            "delivery" to "download",
        ))
        return call("batch.submit_job", body).jsonObject.toJob()
    }

    fun awaitDone(jobId: String): BatchJob {
        val deadline = System.nanoTime() + timeout.toNanos()
        while (true) {
            val job = call("batch.list_jobs", null).jsonArray
                .map { it.jsonObject.toJob() }.firstOrNull { it.id == jobId }
                ?: error("job $jobId vanished from list_jobs")
            if (job.state == "done" || job.state == "expired") return job
            check(System.nanoTime() < deadline) { "job $jobId still '${job.state}' after $timeout" }
            Thread.sleep(pollEvery.toMillis())
        }
    }

    /** Download every file the job produced into workDir/<jobId>/. */
    fun download(jobId: String): List<File> {
        val dir = File(workDir, jobId).apply { mkdirs() }
        return call("batch.list_files", null, mapOf("job_id" to jobId)).jsonArray.mapNotNull { f ->
            val o = f.jsonObject
            val name = o["filename"]?.jsonPrimitive?.content ?: return@mapNotNull null
            val url = o["urls"]?.jsonObject?.get("https")?.jsonPrimitive?.content ?: return@mapNotNull null
            val out = File(dir, name)
            if (!out.exists()) {
                val r = http.send(authed(URI.create(url)).GET().build(),
                    HttpResponse.BodyHandlers.ofFile(out.toPath()))
                check(r.statusCode() == 200) { "download $name -> ${r.statusCode()}" }
            }
            out
        }
    }

    private fun parse(file: File, instruments: Map<String, Long>): List<BarRow> {
        val lines = file.readLines().filter { it.isNotBlank() }
        if (lines.size <= 1) return emptyList()
        val h = lines.first().split(",").withIndex().associate { (i, n) -> n.trim() to i }
        for (c in listOf("ts_event", "symbol", "close")) require(c in h) { "${file.name}: no `$c` column" }
        return lines.drop(1).mapNotNull { line ->
            val p = line.split(",")
            val id = instruments[p[h.getValue("symbol")]] ?: return@mapNotNull null
            fun px(c: String) = h[c]?.let { p[it].toDoubleOrNull() }
            BarRow(
                ts = OffsetDateTime.parse(p[h.getValue("ts_event")]).toLocalDateTime(),
                instrumentId = id,
                open = px("open"), high = px("high"), low = px("low"), close = px("close"),
                volume = h["volume"]?.let { p[it].toLongOrNull() },
            )
        }
    }

    // --- plumbing ----------------------------------------------------------

    private fun JsonObject.toJob() = BatchJob(
        id = this["id"]!!.jsonPrimitive.content,
        state = this["state"]!!.jsonPrimitive.content,
        recordCount = this["record_count"]?.jsonPrimitive?.longOrNull,
        costUsd = this["cost_usd"]?.jsonPrimitive?.doubleOrNull,
    )

    private fun form(m: Map<String, String>) = m.entries.joinToString("&") { (k, v) ->
        "${URLEncoder.encode(k, UTF_8)}=${URLEncoder.encode(v, UTF_8)}"
    }

    private fun authed(uri: URI) = HttpRequest.newBuilder(uri)
        .header("Authorization", "Basic ${Base64.getEncoder().encodeToString("$apiKey:".toByteArray())}")

    private fun call(endpoint: String, body: String?, query: Map<String, String> = emptyMap()): JsonElement {
        val q = if (query.isEmpty()) "" else "?" + form(query)
        val b = authed(URI.create("https://hist.databento.com/v0/$endpoint$q"))
        val req = if (body == null) b.GET().build()
        else b.header("Content-Type", "application/x-www-form-urlencoded")
            .POST(HttpRequest.BodyPublishers.ofString(body)).build()
        val res = http.send(req, HttpResponse.BodyHandlers.ofString())
        check(res.statusCode() == 200) { "databento $endpoint -> ${res.statusCode()}: ${res.body().take(300)}" }
        return Json.parseToJsonElement(res.body())
    }
}
