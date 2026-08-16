package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier

/**
 * One PDF, rendered natively.
 *
 * **No pool and no host.** Every open document is composed as its own viewer and simply stays
 * in the composition — Compose keeps it alive because it is still in the tree, which is the
 * cheapest possible statement of "do not rebuild this". Switching documents changes which one
 * draws on top; nothing is created, destroyed, re-parented or re-laid-out.
 *
 * The caller is responsible for keeping every open document composed. Wrapping this in a
 * condition — `if (active) PdfViewer(…)` — reads like an optimisation and is a teardown, which
 * is the failure this shape exists to make impossible.
 */
@Composable
expect fun PdfViewer(
    filePath: String,
    page: Int,
    onPageChanged: (page: Int) -> Unit,
    modifier: Modifier = Modifier,
)
