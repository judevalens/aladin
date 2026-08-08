package aladin.vendor

import aladin.BarRow
import aladin.DateRange
import aladin.Env
import aladin.Schema

/**
 * Money rails.
 *
 * Two layers, and the cheap one saves the most:
 *
 *  1. **Coverage** already means a held range is never re-fetched. It costs nothing and
 *     never asks, so the best request is the one never made.
 *  2. **This**: for what genuinely must be fetched, price it with the vendor's own
 *     estimate, auto-approve small amounts, and require a typed reply above a threshold.
 *
 * Fails **closed**. No console and no approval — a background job must not spend by
 * defaulting to yes, and must not hang waiting for a reply nobody will type.
 */
fun interface CostApprover {
    /** True to proceed. Called only when a request exceeds the auto-approve threshold. */
    fun approve(description: String, costUsd: Double): Boolean
}

/** Reads a reply from the console; refuses when there is no console rather than assuming. */
object ConsoleApprover : CostApprover {
    override fun approve(description: String, costUsd: Double): Boolean {
        val interactive = System.console() != null || Env["DATABENTO_INTERACTIVE"] == "1"
        if (!interactive) {
            System.err.println(
                "REFUSED: $description would cost \$${"%.4f".format(costUsd)} and needs approval, " +
                    "but there is no interactive console. Re-run interactively, raise " +
                    "autoApproveUnder, or set DATABENTO_INTERACTIVE=1."
            )
            return false
        }
        print("\n  $description\n  estimated cost \$${"%.4f".format(costUsd)} — proceed? [y/N] ")
        System.out.flush()
        return readlnOrNull()?.trim()?.lowercase() in setOf("y", "yes")
    }
}

/**
 * Wraps a fetcher so nothing is bought unpriced.
 *
 * A decorator rather than a convention, for the same reason [LockedFetcher] is: wrap
 * once at construction and no call site can forget it.
 */
class BudgetedFetcher(
    private val delegate: BarFetcher,
    /** Spend up to this without asking. */
    private val autoApproveUnder: Double = 0.10,
    /** Never spend more than this, approved or not. */
    private val hardCeiling: Double = 25.00,
    private val approver: CostApprover = ConsoleApprover,
) : BarFetcher by delegate, AutoCloseable {

    override fun close() = (delegate as? AutoCloseable)?.close() ?: Unit

    /** Total approved and spent this session. */
    var spentUsd: Double = 0.0
        private set

    init {
        require(autoApproveUnder >= 0) { "autoApproveUnder cannot be negative" }
        require(hardCeiling >= autoApproveUnder) {
            "hardCeiling \$$hardCeiling is below autoApproveUnder \$$autoApproveUnder, " +
                "so every request would be refused"
        }
    }

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        val priced = delegate as? PricedFetcher
            ?: return delegate.fetch(instruments, schema, range)   // unpriceable, e.g. a fake

        val cost = runCatching { priced.estimateCostUsd(instruments.keys, schema, range) }
            .getOrElse { throw IllegalStateException("could not price the request; refusing to fetch blind", it) }
        val rows = runCatching { priced.recordCount(instruments.keys, schema, range) }.getOrDefault(-1L)
        val what = "${instruments.size} symbol(s) x ${schema.wire} over $range" +
            if (rows >= 0) "  ($rows records)" else ""

        check(cost <= hardCeiling) {
            "\$${"%.4f".format(cost)} exceeds the hard ceiling of \$${"%.2f".format(hardCeiling)} — " +
                "narrow the range or the universe. $what"
        }
        if (cost > autoApproveUnder && !approver.approve(what, cost)) {
            error("declined: $what (\$${"%.4f".format(cost)})")
        }

        val out = delegate.fetch(instruments, schema, range)
        spentUsd += cost
        return out
    }
}
