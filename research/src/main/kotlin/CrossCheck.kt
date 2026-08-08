import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.time.Instant
import java.util.Base64

private val http: HttpClient = HttpClient.newHttpClient()

private fun databento(dataset: String, day: String): String {
    val auth = Base64.getEncoder().encodeToString("${Env.require("DATABENTO_API_KEY")}:".toByteArray())
    val url = "https://hist.databento.com/v0/timeseries.get_range" +
            "?dataset=$dataset&symbols=AAPL&schema=ohlcv-1d&stype_in=raw_symbol&map_symbols=true" +
            "&start=$day&end=${java.time.LocalDate.parse(day).plusDays(1)}&encoding=csv"
    val r = http.send(HttpRequest.newBuilder(URI.create(url))
        .header("Authorization", "Basic $auth").GET().build(), HttpResponse.BodyHandlers.ofString())
    return if (r.statusCode() == 200) r.body() else "ERR ${r.statusCode()}"
}

private fun alpaca(day: String): String {
    val url = "https://data.alpaca.markets/v2/stocks/AAPL/bars" +
            "?start=$day&end=$day&timeframe=1Day&adjustment=raw&feed=sip"
    val r = http.send(HttpRequest.newBuilder(URI.create(url))
        .header("APCA-API-KEY-ID", Env.require("ALPACA_API_KEY"))
        .header("APCA-API-SECRET-KEY", Env.require("ALPACA_API_SECRET"))
        .GET().build(), HttpResponse.BodyHandlers.ofString())
    return r.body()
}

fun main() {
    val day = "2024-08-01"
    println("AAPL $day — independent reference first\n")

    val a = alpaca(day)
    println("  ALPACA (full consolidated tape, verified 2016+):")
    println("    ${a.take(220)}\n")

    println("  ${"dataset".padEnd(15)}${"bar timestamp (UTC)".padEnd(22)}${"close".padStart(9)}${"volume".padStart(14)}")
    for (d in listOf("EQUS.SUMMARY", "EQUS.MINI", "DBEQ.BASIC", "XNAS.ITCH", "XNYS.PILLAR")) {
        val body = databento(d, day)
        val lines = body.lineSequence().filter { it.isNotBlank() }.toList()
        if (lines.size < 2) { println("  ${d.padEnd(15)}${body.take(50)}"); continue }
        val h = lines[0].split(",").withIndex().associate { (i, n) -> n.trim() to i }
        for (row in lines.drop(1)) {
            val f = row.split(",")
            val ts = Instant.ofEpochSecond(f[h["ts_event"]!!].toLong() / 1_000_000_000L)
            println("  ${d.padEnd(15)}${ts.toString().padEnd(22)}" +
                    "${"%9.2f".format(f[h["close"]!!].toDouble() * 1e-9)}" +
                    "${"%,14d".format(f[h["volume"]!!].toLong())}")
        }
    }
}
