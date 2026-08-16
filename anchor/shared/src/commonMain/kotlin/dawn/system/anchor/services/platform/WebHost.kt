package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.Stable
import androidx.compose.ui.Modifier

/**
 * A live web view, owned **above** the composable that shows it.
 *
 * This split is the whole point. The web view is the longest-lived object in the app — it
 * holds every open document's session, undo history and socket — and it must not share a
 * lifetime with the slot that happens to be displaying it. A recycler disposes its pages;
 * this survives that, and the page re-attaches the same instance when it comes back.
 *
 * Create it once with [rememberWebHost] at a scope that outlives the recycler, and attach
 * it wherever it should appear with [WebHostSurface]. Attaching in two places at once is a
 * programming error: a UIView has one superview.
 */
@Stable
expect class WebHostHandle

/**
 * Creates the web view and loads [filePath], once. Call this **outside** any recycler.
 *
 * [bootstrapJs] runs at document start, before the page's own scripts — how the page
 * receives session configuration (bearer token, collab URL, user) without putting a token
 * in a URL, where it would leak into history, logs and referrers. It is read once at
 * creation; changing it later does not rebuild the view, deliberately, because rebuilding
 * would close every open editor.
 *
 * [onMessage] receives the page's messages verbatim; parsing belongs to the caller, which
 * is the side that knows the protocol.
 */
@Composable
expect fun rememberWebHost(
    filePath: String,
    readAccessDirPath: String,
    bootstrapJs: String,
    onMessage: (String) -> Unit,
    onContentProcessTerminated: () -> Unit,
): WebHostHandle

/**
 * Shows [handle] here. Composing this attaches the view; leaving composition detaches it
 * without destroying it.
 *
 * [command] is JavaScript re-evaluated whenever it changes, and again after every completed
 * navigation — so the caller describes *desired state* and the page diffs it. It must be
 * **idempotent**: it is re-sent after a reload, and a web content process can die at any
 * moment.
 */
@Composable
expect fun WebHostSurface(
    handle: WebHostHandle,
    command: String,
    modifier: Modifier = Modifier,
)
