/** What am I entitled to, and how far back does each go? Metadata calls are free. */

import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.util.Base64

private fun api(path: String): String {
    val auth = Base64.getEncoder().encodeToString("${Env.require("DATABENTO_API_KEY")}:".toByteArray())
    val res = HttpClient.newHttpClient().send(
        HttpRequest.newBuilder(URI.create("https://hist.databento.com/v0/$path"))
            .header("Authorization", "Basic $auth").GET().build(),
        HttpResponse.BodyHandlers.ofString(),
    )
    return if (res.statusCode() == 200) res.body() else "ERR ${res.statusCode()}: ${res.body().take(120)}"
}

fun main() {
    val datasets = api("metadata.list_datasets")
        .removeSurrounding("[", "]").split(",")
        .map { it.trim().trim('"') }.filter { it.isNotBlank() }

    println("${datasets.size} datasets available\n")
    println("  ${"dataset".padEnd(18)}available range")
    for (d in datasets) {
        val range = api("metadata.get_dataset_range?dataset=$d")
            .replace("\"", "").replace("{", "").replace("}", "")
        println("  ${d.padEnd(18)}$range")
    }
}
