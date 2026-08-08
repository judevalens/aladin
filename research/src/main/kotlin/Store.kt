/**
 * The store interface: ask for bars, get bars.
 *
 *     BarStore.databento().use { store ->
 *         val bars = store.bars(listOf("AAPL", "MSFT"), range)
 *     }
 *
 * One call. Held ranges come from disk; anything missing is fetched, priced and
 * approved on the way. The caller never sees a connection, a fetcher, an
 * instrument id or a source string — those are the things that were getting
 * mis-wired when they were the caller's job.
 */

import org.jetbrains.kotlinx.dataframe.AnyFrame
import java.io.Closeable
import java.sql.Connection
import java.time.LocalDate

/** What a strategy or a notebook depends on. Deliberately small. */
interface Bars {
    fun bars(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate = range.from,
        schema: Schema = Schema.OHLCV_1D,
        field: String = "close",
        holes: Holes = Holes.NAN,
    ): BarMatrix

    /** The same query as a DataFrame — for looking, not for the engine's hot path. */
    fun frame(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate = range.from,
        schema: Schema = Schema.OHLCV_1D,
        field: String = "close",
    ): AnyFrame
}

class BarStore(
    private val conn: Connection,
    private val fetcher: BarFetcher?,
    /** Where bars are read from. Defaults to the fetcher's scope, or the only one held. */
    private val source: String = fetcher?.source ?: conn.soleSource(),
) : Bars, Closeable {

    init {
        conn.createInstrumentsTable(); conn.createOhlcvTable()
        conn.createCoverageTable(); conn.createSymbologyTable()
    }

    override fun bars(
        symbols: List<String>, range: DateRange, asOf: LocalDate,
        schema: Schema, field: String, holes: Holes,
    ): BarMatrix {
        fill(symbols, range, asOf, schema)
        return conn.loadMatrix(symbols, range, asOf, field, schema, source, holes)
    }

    override fun frame(
        symbols: List<String>, range: DateRange, asOf: LocalDate, schema: Schema, field: String,
    ): AnyFrame {
        fill(symbols, range, asOf, schema)
        return conn.loadFrame(symbols, range, asOf, field, schema, source)
    }

    /** What this store actually holds, per instrument — for deciding what to ask for. */
    fun held(schema: Schema = Schema.OHLCV_1D): AnyFrame = conn.frame("""
        SELECT i.symbol, min(o.ts)::DATE AS first, max(o.ts)::DATE AS last, count(*) AS bars
        FROM ohlcv o JOIN instruments i ON i.instrument_id = o.instrument_id
        WHERE o.source = '$source' AND o.schema = '${schema.wire}'
        GROUP BY i.symbol ORDER BY i.symbol""")

    /** Approved spend so far, when the fetcher is budgeted. */
    val spentUsd: Double get() = (fetcher as? BudgetedFetcher)?.spentUsd ?: 0.0

    override fun close() = conn.close()

    // --- internals ---------------------------------------------------------

    /** Resolve identity, then fetch whatever the coverage ledger says is missing. */
    private fun fill(symbols: List<String>, range: DateRange, asOf: LocalDate, schema: Schema) {
        val f = fetcher ?: return          // read-only store: whatever is held is all there is
        conn.ensureBars(f, identify(symbols, asOf), schema, range)
    }

    /**
     * A symbol the registry has never seen gets an id here.
     *
     * KNOWN GAP: the validity window is open, because without vendor symbology we do
     * not know when the instrument actually listed. That makes as-of resolution
     * optimistic for minted instruments — it will never say "did not exist then". Real
     * validity needs `symbology.resolve`, which is also what D6 needs.
     */
    private fun identify(symbols: List<String>, asOf: LocalDate): Map<String, Long> =
        symbols.associateWith { sym ->
            conn.resolveInstrument(sym, asOf) ?: run {
                val id = conn.createStatement().use { st ->
                    st.executeQuery("SELECT coalesce(max(instrument_id), 0) + 1 FROM instruments")
                        .use { it.next(); it.getLong(1) }
                }
                conn.registerInstrument(Instrument(id, sym, PROVISIONAL_FROM, null))
                id
            }
        }

    companion object {
        private val PROVISIONAL_FROM: LocalDate = LocalDate.parse("1900-01-01")

        /** Read-only over whatever is already on disk. Never fetches, never spends. */
        fun readOnly(path: String = RESEARCH_DB): BarStore = BarStore(openDb(path), fetcher = null)

        /** Backed by Databento, with the budget gate wired in. */
        fun databento(
            path: String = RESEARCH_DB,
            autoApproveUnder: Double = (Env["DATABENTO_AUTO_APPROVE_UNDER"] ?: "0.10").toDouble(),
            hardCeiling: Double = (Env["DATABENTO_HARD_CEILING"] ?: "5.00").toDouble(),
            batch: Boolean = true,
        ): BarStore = BarStore(
            openDb(path),
            BudgetedFetcher(
                if (batch) DatabentoBatchFetcher() else DatabentoFetcher(),
                autoApproveUnder = autoApproveUnder,
                hardCeiling = hardCeiling,
            ),
        )
    }
}

/** The single source held, so a read-only store needs no ceremony to open. */
private fun Connection.soleSource(): String =
    createStatement().use { st ->
        st.executeQuery("SELECT DISTINCT source FROM ohlcv").use { rs ->
            val all = buildList { while (rs.next()) add(rs.getString(1)) }
            when (all.size) {
                1 -> all.single()
                0 -> "databento:EQUS.SUMMARY"
                else -> error("store holds ${all.size} sources $all — pass one explicitly")
            }
        }
    }
