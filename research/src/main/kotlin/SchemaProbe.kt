fun main() {
    println("resolution      bars/session   vs daily   500 syms x 10yr")
    for (s in Schema.entries) {
        val rows = s.rowsFor(instruments = 500, sessions = 2520)
        println("  ${s.wire.padEnd(12)}${s.barsPerSession.toString().padStart(8)}" +
                "${"%9.1fx".format(s.vsDaily)}   ${"%,15d".format(rows)} rows")
    }
    println("\ntradeable: ${InstrumentType.entries.filter { it.tradeable }}")
    println("not:       ${InstrumentType.entries.filterNot { it.tradeable }}")
}
