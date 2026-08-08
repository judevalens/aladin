package aladin.vendor

import aladin.Env
import java.io.IOException
import java.net.URI
import java.net.URLEncoder
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Duration
import java.util.Base64
import kotlin.text.Charsets.UTF_8

/**
 * Shared HTTP for vendor calls.
 *
 * Three things a bare [HttpClient] does not do, each of which has a real failure mode:
 *
 *  - **Timeouts.** The default client waits forever. A hung request would block a
 *    backtest indefinitely with no indication why.
 *  - **Retry.** Rate limits and 5xx are transient; failing the whole fetch on one is
 *    wasteful when the range has already been priced. `Retry-After` is honoured, since
 *    guessing a backoff against a rate limiter is how you get limited harder.
 *  - **Not retrying 4xx.** A bad request will fail identically every time; retrying it
 *    just burns quota and delays the error the caller needs to see.
 */
class VendorHttp(
    private val apiKey: String,
    private val baseUrl: String,
    private val maxAttempts: Int = 5,
    connectTimeout: Duration = Duration.ofSeconds(10),
    private val requestTimeout: Duration = Duration.ofMinutes(2),
    private val client: HttpClient = HttpClient.newBuilder().connectTimeout(connectTimeout).build(),
    private val sleep: (Duration) -> Unit = { Thread.sleep(it.toMillis()) },
) {
    private val auth = "Basic " + Base64.getEncoder().encodeToString("$apiKey:".toByteArray())

    fun get(endpoint: String, params: Map<String, String> = emptyMap()): String =
        send(endpoint, params, post = false)

    fun post(endpoint: String, params: Map<String, String>): String =
        send(endpoint, params, post = true)

    /** Stream a URL straight to a file, for batch downloads. */
    fun download(url: String, into: java.io.File) {
        val res = client.send(
            HttpRequest.newBuilder(URI.create(url))
                .header("Authorization", auth).timeout(requestTimeout).GET().build(),
            HttpResponse.BodyHandlers.ofFile(into.toPath()),
        )
        check(res.statusCode() == 200) { "download ${into.name} -> ${res.statusCode()}" }
    }

    private fun send(endpoint: String, params: Map<String, String>, post: Boolean): String {
        val form = params.entries.joinToString("&") { (k, v) ->
            "${URLEncoder.encode(k, UTF_8)}=${URLEncoder.encode(v, UTF_8)}"
        }
        var lastError: String? = null

        repeat(maxAttempts) { attempt ->
            val builder = HttpRequest.newBuilder(
                URI.create("$baseUrl/$endpoint" + if (post || form.isEmpty()) "" else "?$form")
            ).header("Authorization", auth).timeout(requestTimeout)

            val request =
                if (post) builder.header("Content-Type", "application/x-www-form-urlencoded")
                    .POST(HttpRequest.BodyPublishers.ofString(form)).build()
                else builder.GET().build()

            val res = try {
                client.send(request, HttpResponse.BodyHandlers.ofString())
            } catch (e: IOException) {
                lastError = "${e::class.simpleName}: ${e.message}"
                if (attempt < maxAttempts - 1) sleep(backoff(attempt, null))
                return@repeat
            }

            when {
                res.statusCode() == 200 -> return res.body()

                // transient — worth another attempt
                res.statusCode() == 429 || res.statusCode() >= 500 -> {
                    lastError = "${res.statusCode()}: ${res.body().take(200)}"
                    if (attempt < maxAttempts - 1) {
                        sleep(backoff(attempt, res.headers().firstValue("Retry-After").orElse(null)))
                    }
                }

                // a bad request fails the same way every time; surface it immediately
                else -> error("$endpoint -> ${res.statusCode()}: ${res.body().take(300)}")
            }
        }
        error("$endpoint failed after $maxAttempts attempts. Last: $lastError")
    }

    /** Honour Retry-After when given; otherwise exponential, capped. */
    private fun backoff(attempt: Int, retryAfter: String?): Duration =
        retryAfter?.toLongOrNull()?.let(Duration::ofSeconds)
            ?: Duration.ofMillis(minOf(500L shl attempt, 30_000L))

    companion object {
        const val DATABENTO_HIST = "https://hist.databento.com/v0"

        fun databento(apiKey: String = Env.require("DATABENTO_API_KEY")): VendorHttp =
            VendorHttp(apiKey, DATABENTO_HIST)
    }
}
