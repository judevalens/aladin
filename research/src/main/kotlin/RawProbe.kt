import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.util.Base64

private fun csv(dataset: String, day: String): String {
    val auth = Base64.getEncoder().encodeToString("${Env.require("DATABENTO_API_KEY")}:".toByteArray())
    val url = "https://hist.databento.com/v0/timeseries.get_range" +
            "?dataset=$dataset&symbols=AAPL&schema=ohlcv-1d&stype_in=raw_symbol&map_symbols=true" +
            "&start=$day&end=${java.time.LocalDate.parse(day).plusDays(1)}&encoding=csv"   // end is exclusive
    val r = HttpClient.newHttpClient().send(
        HttpRequest.newBuilder(URI.create(url)).header("Authorization", "Basic $auth").GET().build(),
        HttpResponse.BodyHandlers.ofString())
    return if (r.statusCode() == 200) r.body() else "ERR ${r.statusCode()}"
}

fun main() {
    println("AAPL daily bar, same session, across datasets (real consolidated volume ~80M):\n")
    println("  ${"dataset".padEnd(16)}${"close".padStart(9)}${"volume".padStart(14)}  columns")
    for (d in listOf("EQUS.MINI", "EQUS.SUMMARY", "DBEQ.BASIC", "XNAS.ITCH", "XNYS.PILLAR")) {
        val body = csv(d, "2024-08-01")
        val lines = body.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size < 2) { println("  ${d.padEnd(16)}${" ".padStart(9)}${"—".padStart(14)}  ${body.take(40)}"); continue }
        val h = lines[0].split(",").withIndex().associate { (i, n) -> n.trim() to i }
        val f = lines[1].split(",")
        val close = h["close"]?.let { f[it].toDouble() * 1e-9 }
        val vol = h["volume"]?.let { f[it].toLong() }
        println("  ${d.padEnd(16)}${"%9.2f".format(close)}${"%,14d".format(vol)}  ${lines[0].take(60)}")
    }
}
