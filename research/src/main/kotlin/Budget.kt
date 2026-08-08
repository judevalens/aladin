/**
 * Money rails.
 *
 * Two layers, and the cheap one matters most:
 *
 *  1. **Coverage** already means a held range is never re-fetched. That is the
 *     rail that saves the most, because it costs nothing and never asks.
 *  2. **This**: for what genuinely has to be fetched, price it with the vendor's
 *     own estimate first, auto-approve small amounts, and require a typed reply
 *     above a threshold.
 *
 * Fails CLOSED. No console, no approval — a background job cannot spend money by
 * silently defaulting to yes, and it must not hang waiting for a reply nobody
 * will type.
 */

import java.time.Duration

/** A fetcher that can price a request before making it. */
interface PricedFetcher : BarFetcher {
    fun estimateCostUsd(symbols: Collection<String>, schema: Schema, range: DateRange): Double
    fun recordCount(symbols: Collection<String>, schema: Schema, range: DateRange): Long
}

fun interface CostApprover {
    /** True to proceed. Called only when a request exceeds the auto-approve threshold. */
    fun approve(description: String, costUsd: Double): Boolean
}

/**
 * Reads a reply from the console. Returns false when there is no console, so an
 * unattended run refuses rather than blocking or assuming consent.
 */
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
 * Wraps a fetcher so nothing is bought without being priced first.
 *
 * Wrap once, at construction, and the gate cannot be forgotten at a call site —
 * the same reason [LockedFetcher] is a decorator rather than a convention.
 */
class BudgetedFetcher(
    private val delegate: BarFetcher,
    /** Spend up to this without asking. */
    private val autoApproveUnder: Double = 0.10,
    /** Never spend more than this, approved or not. */
    private val hardCeiling: Double = 25.00,
    private val approver: CostApprover = ConsoleApprover,
) : BarFetcher by delegate {

    /** Total actually approved and spent this session. */
    var spentUsd: Double = 0.0
        private set

    override fun fetch(instruments: Map<String, Long>, schema: Schema, range: DateRange): List<BarRow> {
        val priced = delegate as? PricedFetcher
            ?: return delegate.fetch(instruments, schema, range)   // unpriceable (e.g. a fake)

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
