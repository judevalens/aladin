import java.time.LocalDate
import java.util.concurrent.atomic.AtomicInteger

/** Stands in for Databento symbology; counts calls so we can prove it's asked once. */
private class FakeSymbology : SymbologySource {
    override val source = "fake"
    val calls = AtomicInteger()
    override fun history(symbol: String): List<Instrument> {
        calls.incrementAndGet()
        return when (symbol) {
            "NVDA" -> listOf(Instrument(101L, "NVDA", LocalDate.parse("1999-01-22"), null))
            "SIVB" -> listOf(Instrument(102L, "SIVB", LocalDate.parse("1988-01-01"), LocalDate.parse("2023-05-01")))
            else -> emptyList()                       // vendor knows nothing about it
        }
    }
}

fun main() {
    val sym = FakeSymbology()
    openScratch().use { c ->
        c.createStatement().use {
            it.execute("DROP TABLE IF EXISTS instruments"); it.execute("DROP TABLE IF EXISTS symbology_checked")
        }
        c.createInstrumentsTable(); c.createSymbologyTable()

        fun ask(label: String, s: String, d: String) {
            val before = sym.calls.get()
            val id = c.resolveOrFetch(s, LocalDate.parse(d), sym)
            println("  ${label.padEnd(34)} id=${(id?.toString() ?: "none").padEnd(6)} " +
                    "vendor calls this ask=${sym.calls.get() - before}")
        }

        println("nothing registered up front — the registry fills on demand:")
        ask("NVDA as-of 2024-01-02", "NVDA", "2024-01-02")
        ask("NVDA again", "NVDA", "2024-01-02")
        ask("NVDA at a different date", "NVDA", "2020-06-01")

        println("\ndelisted: exists before, not after — and only ever asked once:")
        ask("SIVB as-of 2022-01-03", "SIVB", "2022-01-03")
        ask("SIVB as-of 2024-01-02", "SIVB", "2024-01-02")
        ask("SIVB as-of 2024-06-01", "SIVB", "2024-06-01")

        println("\nunknown ticker — the negative is remembered too:")
        ask("NOPE as-of 2024-01-02", "NOPE", "2024-01-02")
        ask("NOPE again", "NOPE", "2024-01-02")
        println("\n  total vendor calls: ${sym.calls.get()}  (3 symbols, asked 8 times)")
    }
}
