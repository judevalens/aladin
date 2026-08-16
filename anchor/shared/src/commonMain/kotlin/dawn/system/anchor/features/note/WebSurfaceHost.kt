package dawn.system.anchor.features.note

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import anchor.shared.generated.resources.Res
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.network.ApiConfig
import androidx.compose.runtime.Stable
import dawn.system.anchor.services.platform.WebHostHandle
import dawn.system.anchor.services.platform.WebHostSurface
import dawn.system.anchor.services.platform.rememberWebHost
import dawn.system.anchor.services.platform.documentsDirPath
import dawn.system.anchor.services.platform.fileExists
import dawn.system.anchor.services.platform.fileSizeBytes
import dawn.system.anchor.services.platform.writeBytes
import org.jetbrains.compose.resources.ExperimentalResourceApi

/** One web-backed artifact open inside the host. */
data class WebPane(val id: String, val kind: Kind, val title: String) {
    enum class Kind(val wire: String) {
        /** A BlockNote page, editing the same Y.Doc desktop is editing. */
        Page("page"),

        /** An agent-authored shard, sandboxed to an opaque origin. */
        Shard("shard"),
    }
}

/**
 * The companion's **one** web view, and everything web-backed inside it.
 *
 * Reimplementing a block editor and a Yjs client in Kotlin would be weeks of work for a
 * *second* implementation of the block model, to be kept in step with desktop forever.
 * Instead the companion carries the real editor as a self-contained document
 * (`aladin_react`'s `npm run build:embed`) and loads it from `file://`.
 *
 * **Why one view rather than one per artifact.** A WebContent process is ~187 MB, so a view
 * per open note capped the companion at about two of them and made every switch a native
 * hide/reveal — which reclaims nothing and repaints visibly. Here the view is mounted once
 * for the life of the shell and told what to show; a second open note costs a Y.Doc and a
 * socket, and switching between notes never touches the platform view at all.
 *
 * The bridge is deliberately **declarative**: the command carries the whole desired state,
 * so a dropped or repeated evaluation cannot leave the two sides disagreeing about what is
 * open. That matters more than it looks — a web content process can die at any moment, and
 * recovery is then just "send the state again".
 *
 * **The iPad is a full client.** Nothing here fetches the editor's code at runtime — no dev
 * server, no deployed web app. The only thing it needs on the network is the collab server,
 * which is inherent: a shared document is a network session, and a native editor would need
 * it too.
 */
/** The host, once it has a document to load. Null while the bundle is being unpacked. */
@Stable
data class WebSurfaceHost(val handle: WebHostHandle, val restarts: Int)

/**
 * Creates the one web view, at a scope that outlives the pager.
 *
 * Everything here happens once per launch: unpack the bundle to disk (WKWebView needs a real
 * path, and Compose resources live inside the app bundle), then build the view. The result
 * is a handle the pager's web slot attaches — never owns.
 */
@OptIn(ExperimentalResourceApi::class)
@Composable
fun rememberWebSurfaceHost(
    token: String?,
    userName: String?,
    config: ApiConfig,
): WebSurfaceHost? {
    var documentPath by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        documentPath = runCatching { unpackEditorDocument { Res.readBytes(EDITOR_RESOURCE) } }
            .getOrNull()
    }

    // A dead web content process leaves a blank pane and nothing else; surface it. The
    // command is re-evaluated on the reload that follows, which is why it has to describe
    // state rather than a sequence of operations.
    var restarts by remember { mutableStateOf(0) }

    val path = documentPath ?: return null

    // The bootstrap is read once, when the view is built. A token refresh mid-session
    // therefore does not reach the collab connection — a real gap, but a far smaller one
    // than making the token part of this view's identity, which would close every open
    // editor the moment it rotated.
    val handle = rememberWebHost(
        filePath = path,
        readAccessDirPath = editorDirPath(),
        bootstrapJs = embedBootstrap(
            token = token.orEmpty(),
            collabWsUrl = config.collabWsUrl,
            userName = userName ?: DEFAULT_USER_NAME,
        ),
        onMessage = { /* `ready` only, today; the page queues its own backlog. */ },
        onContentProcessTerminated = { restarts++ },
    )

    return remember(handle, restarts) { WebSurfaceHost(handle, restarts) }
}

/**
 * The pager's web page: attaches the shared view and tells it what to show.
 *
 * `imePadding` here and NOT on the shell's columns — the editor is the only surface that
 * takes text, so it is the only one that should give way to the keyboard. WebKit then
 * scrolls the caret within the smaller viewport, which is one thing moving instead of two.
 */
@Composable
fun WebSurfacePage(
    host: WebSurfaceHost,
    panes: List<WebPane>,
    activeId: String?,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors

    Box(modifier.fillMaxSize().background(c.bg)) {
        WebHostSurface(
            handle = host.handle,
            command = syncCommand(panes, activeId),
            modifier = Modifier.fillMaxSize().imePadding(),
        )

        if (host.restarts > 0) {
            Text(
                "the editor's web process restarted",
                style = MetaStyle,
                color = c.ink4,
                modifier = Modifier.align(Alignment.BottomStart).padding(14.dp),
            )
        }
    }
}

private const val DEFAULT_USER_NAME = "iPad"
private const val EDITOR_RESOURCE = "files/page-editor.html"
private const val EDITOR_FILE = "page-editor.html"

internal fun editorDirPath(): String = "${documentsDirPath()}/editor"

/**
 * Writes the bundled editor to disk and returns its path.
 *
 * **Rewritten whenever the bundle changes**, which is not a detail. iOS keeps an app's data
 * container across a rebuild-and-run, so a version that only wrote the file when it was
 * missing would pin every already-installed device to whatever editor it first unpacked —
 * you rebuild the bundle, reinstall, and the iPad still runs the old one, with nothing on
 * screen to say why.
 *
 * Size is the staleness check: the bundle is one generated artifact, so any real change
 * moves it, and comparing costs one local read of something already in the app.
 *
 * [readResource] is a parameter so the unpack rule is testable without Compose resources.
 */
internal suspend fun unpackEditorDocument(readResource: suspend () -> ByteArray): String {
    val path = "${editorDirPath()}/$EDITOR_FILE"
    val bytes = readResource()
    if (!fileExists(path) || fileSizeBytes(path) != bytes.size.toLong()) {
        writeBytes(path, bytes)
    }
    return path
}

/** Session configuration, as one global set before the page's own scripts run. */
internal fun embedBootstrap(
    token: String,
    collabWsUrl: String,
    userName: String,
): String = "window.__ALADIN_EMBED__={" +
    "token:${jsString(token)}," +
    "collabWsUrl:${jsString(collabWsUrl)}," +
    "user:{name:${jsString(userName)},color:${jsString(colorForUser(userName))}}" +
    "};" +
    // The stub queues whatever arrives before React mounts, then the app drains it. Without
    // it the native side would have to wait for a "ready" message before asking for
    // anything, and the first note would open a round trip later than it needs to.
    "window.__aladinHost=window.__aladinHost||{sync:function(s){this._queued=s;}};"

/**
 * The whole desired state, as one idempotent call.
 *
 * Not a diff and not a sequence of opens and closes: those need the two sides to agree on
 * history, and after a web content process dies there is no history to agree on.
 */
internal fun syncCommand(panes: List<WebPane>, activeId: String?): String {
    val list = panes.joinToString(",") { pane ->
        "{id:${jsString(pane.id)},kind:${jsString(pane.kind.wire)},title:${jsString(pane.title)}}"
    }
    val active = activeId?.takeIf { id -> panes.any { it.id == id } }
    return "window.__aladinHost.sync({panes:[$list],active:${active?.let(::jsString) ?: "null"}});"
}

/**
 * Escapes a value for embedding in a script literal.
 *
 * Load-bearing, not defensive dressing: a token or title containing a quote, backslash or
 * newline would otherwise produce a syntax error and a blank page — miserable to debug
 * through a web view. `<` is escaped so a value can never close the surrounding element,
 * and U+2028/U+2029 because JavaScript treats them as line terminators inside string
 * literals even though JSON does not.
 */
internal fun jsString(value: String): String = buildString {
    append('"')
    for (ch in value) {
        when {
            ch == '"' -> append("\\\"")
            ch == '\\' -> append("\\\\")
            ch == '\n' -> append("\\n")
            ch == '\r' -> append("\\r")
            ch == '\t' -> append("\\t")
            ch == '<' -> append("\\u003c")
            ch.code == 0x2028 || ch.code == 0x2029 || ch.code < 0x20 ->
                append("\\u" + ch.code.toString(16).padStart(4, '0'))
            else -> append(ch)
        }
    }
    append('"')
}

/**
 * A stable presence colour per user, so a collaboration cursor keeps its identity between
 * sessions rather than changing colour on every reconnect.
 */
internal fun colorForUser(seed: String): String {
    var hash = 0
    for (ch in seed) {
        hash = (hash * 31 + ch.code) and 0x7fffffff
    }
    return "hsl(${hash % 360} 70% 60%)"
}

@Composable
private fun NoteNotice(title: String, detail: String, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    Column(
        modifier.fillMaxSize().background(c.bg).padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, style = MaterialTheme.typography.titleMedium, color = c.ink3)
        Spacer(Modifier.height(6.dp))
        Text(detail, style = MetaStyle, color = c.ink4)
    }
}
