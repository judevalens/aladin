package aladin.store

import aladin.DateRange
import aladin.Env
import aladin.Instrument
import aladin.Schema
import aladin.normalizeSymbols
import aladin.vendor.BarFetcher
import aladin.vendor.BudgetedFetcher
import aladin.vendor.DatabentoBatchFetcher
import aladin.vendor.DatabentoFetcher
import aladin.vendor.DatabentoSymbology
import aladin.vendor.SymbologySource
import aladin.vendor.VendorHttp
import org.jetbrains.kotlinx.dataframe.AnyFrame
import java.io.Closeable
import java.sql.Connection
import java.time.LocalDate

/**
 * What a strategy or a notebook depends on. Deliberately small.
 *
 * An interface rather than a concrete type so a backtest can be handed a canned
 * [BarMatrix] with no database and no vendor, and so a second implementation — a remote
 * store the Go backend queries, say — can arrive without touching strategy code.
 */
interface Bars {
    fun bars(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate = range.from,
        schema: Schema = Schema.OHLCV_1D,
        field: String = "close",
        holes: Holes = Holes.NAN,
    ): BarMatrix

    /** The whole bar as a DataFrame — every column, for looking rather than looping. */
    fun frame(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate = range.from,
        schema: Schema = Schema.OHLCV_1D,
    ): AnyFrame
}

/**
 * The store: ask for bars, get bars.
 *
 *     BarStore.databento().use { store ->
 *         val bars = store.bars(listOf("AAPL", "MSFT"), range)
 *     }
 *
 * Held ranges come off disk; anything missing is resolved, priced, approved and fetched
 * on the way. The caller never sees a connection, a fetcher, an instrument id or a
 * source string — those were exactly the things getting mis-wired while they were the
 * caller's job.
 */
class BarStore(
    private val conn: Connection,
    private val fetcher: BarFetcher? = null,
    private val symbology: SymbologySource? = null,
    /** Which scope to read. Defaults to the fetcher's, or the only one held. */
    private val source: String = fetcher?.source ?: conn.soleSource(),
) : Bars, Closeable {

    init {
        conn.createInstrumentsTable()
        conn.createOhlcvTable()
        conn.createCoverageTable()
        conn.createSymbologyTable()
    }

    override fun bars(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate,
        schema: Schema,
        field: String,
        holes: Holes,
    ): BarMatrix = conn.loadMatrix(
        normalizeSymbols(symbols), range, asOf, field, schema, source, holes, fetcher, symbology,
    )

    override fun frame(
        symbols: List<String>,
        range: DateRange,
        asOf: LocalDate,
        schema: Schema,
    ): AnyFrame = conn.loadFrame(
        normalizeSymbols(symbols), range, asOf, schema, source, fetcher, symbology,
    )

    /** What this store holds, per instrument — for deciding what to ask for. */
    fun held(schema: Schema = Schema.OHLCV_1D): AnyFrame = conn.frame(
        """
        SELECT i.symbol, min(o.ts)::DATE AS first, max(o.ts)::DATE AS last, count(*) AS bars
        FROM ohlcv o JOIN instruments i ON i.instrument_id = o.instrument_id
        WHERE o.source = ${sqlLiteral(source)} AND o.schema = ${sqlLiteral(schema.wire)}
        GROUP BY i.symbol ORDER BY i.symbol
        """
    )

    /**
     * Instruments whose history looks like two different companies sharing a ticker.
     *
     * A smoke alarm rather than a resolver — the store keys on `instrument_id` so a
     * recycled ticker cannot merge two companies, but that only holds while the vendor's
     * identity layer is right. When it isn't, this is what notices.
     */
    fun identityBreaks(
        schema: Schema = Schema.OHLCV_1D,
        minGapDays: Long = 90,
        minRatio: Double = 2.0,
    ): List<IdentityBreak> = conn.identityBreaks(source, schema, minGapDays, minRatio)

    /** Register an instrument whose identity is known without asking a vendor. */
    fun register(instrument: Instrument): BarStore = apply { conn.registerInstrument(instrument) }

    /** Approved spend so far, when the fetcher is budgeted. */
    val spentUsd: Double get() = (fetcher as? BudgetedFetcher)?.spentUsd ?: 0.0

    /**
     * Releases the connection and anything the store owns.
     *
     * The vendor clients hold HttpClient threads; leaving them running keeps the JVM
     * alive after main returns, and a lingering JVM holds the DuckDB file lock — which
     * surfaces on the *next* run as a confusing conflict with a stale PID.
     */
    override fun close() {
        (fetcher as? AutoCloseable)?.runCatching { close() }
        (symbology as? AutoCloseable)?.runCatching { close() }
        conn.close()
    }

    companion object {
        /** Read-only over what is already on disk. Never fetches, never spends. */
        fun readOnly(path: String = RESEARCH_DB): BarStore =
            BarStore(openDb(path), fetcher = null)

        /**
         * Backed by Databento, with the budget gate and vendor symbology wired in.
         *
         * [batch] chooses the transport: batch suits bulk, streaming suits read-through
         * gaps where the answer is wanted now.
         */
        fun databento(
            path: String = RESEARCH_DB,
            autoApproveUnder: Double = Env.double("DATABENTO_AUTO_APPROVE_UNDER", 0.10),
            hardCeiling: Double = Env.double("DATABENTO_HARD_CEILING", 5.00),
            batch: Boolean = true,
        ): BarStore {
            // one HTTP client for all three, so there is one pool to close rather than three
            val http = VendorHttp.databento()
            val dataset = Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY"
            return BarStore(
                openDb(path),
                BudgetedFetcher(
                    if (batch) DatabentoBatchFetcher(dataset, http = http)
                    else DatabentoFetcher(dataset, http = http),
                    autoApproveUnder = autoApproveUnder,
                    hardCeiling = hardCeiling,
                ),
                symbology = DatabentoSymbology(dataset, http),
            )
        }
    }
}

/**
 * The single scope held, so a read-only store needs no ceremony.
 *
 * More than one is ambiguous rather than a default: reading consolidated bars when you
 * meant single-venue produces plausible wrong numbers.
 */
private fun Connection.soleSource(): String {
    createOhlcvTable()
    return createStatement().use { st ->
        st.executeQuery("SELECT DISTINCT source FROM ohlcv").use { rs ->
            val all = buildList { while (rs.next()) add(rs.getString(1)) }
            when (all.size) {
                1 -> all.single()
                0 -> "databento:${Env["DATABENTO_DATASET"] ?: "EQUS.SUMMARY"}"
                else -> error("store holds ${all.size} scopes $all — pass one explicitly")
            }
        }
    }
}
