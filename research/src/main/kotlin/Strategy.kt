import kotlin.reflect.full.primaryConstructor

/**
 * A read-only view of bars up to and including time T.
 *
 * This is the piece Julia couldn't give us. The backing array is private and every
 * access goes through [back], which is bounds-checked against [length] — so bar T+1
 * is genuinely *unreachable*, not merely undocumented. Runtime enforcement rather
 * than a compile-time proof, but a real boundary either way.
 *
 * Because the guarantee is structural, there is no chunk-size dial: the engine can
 * hand the strategy one window per bar and nothing can read past it.
 */
class Window internal constructor(
    private val data: DoubleArray,   // column-major: data[t + sym * rows]
    private val rows: Int,
    private val endExclusive: Int,   // T + 1
    val symbols: Int,
    val length: Int,                 // == strategy lookback
) {
    /** Close `i` bars back from T (0 == the bar at T). */
    fun back(i: Int, sym: Int): Double {
        require(i in 0 until length) { "window holds $length bars; asked for $i back" }
        require(sym in 0 until symbols) { "no symbol $sym (have $symbols)" }
        return data[(endExclusive - 1 - i) + sym * rows]
    }
}

interface Strategy {
    /** Bars of history [signals] needs before its first valid output. A contract, not a hint. */
    val lookback: Int

    /**
     * Write target weights for the last bar of [w] into [out].
     * Weights are what to *hold*, not what to *do*. [out] is reused every bar.
     */
    fun signals(w: Window, out: DoubleArray)
}

/** Param names, types and defaults, straight off the data class — nothing hand-written. */
inline fun <reified P : Any> describe(): Map<String, Pair<String, Any?>> {
    val ctor = P::class.primaryConstructor ?: return emptyMap()
    val defaults = P::class.constructors.firstOrNull { it.parameters.all(kotlin.reflect.KParameter::isOptional) }
        ?.callBy(emptyMap())
    return ctor.parameters.associate { p ->
        val default = defaults?.let { d ->
            P::class.members.firstOrNull { it.name == p.name }?.call(d)
        }
        p.name!! to (p.type.toString().substringAfterLast('.') to default)
    }
}

/**
 * Walk bars, handing [strat] one window at a time. Returns weights for every bar
 * from `lookback` onward, plus the row index the first output corresponds to.
 */
fun runStrategy(
    strat: Strategy,
    close: DoubleArray,
    rows: Int,
    symbols: Int,
): Pair<Array<DoubleArray>, Int> {
    val l = strat.lookback
    require(l > 0) { "lookback must be positive" }
    require(rows >= l) { "need at least $l bars, got $rows" }

    val out = Array(rows - l + 1) { DoubleArray(symbols) }
    val w = DoubleArray(symbols)

    for (t in l - 1 until rows) {
        val window = Window(close, rows, t + 1, symbols, l)
        strat.signals(window, w)
        w.copyInto(out[t - l + 1])
    }
    return out to (l - 1)
}
