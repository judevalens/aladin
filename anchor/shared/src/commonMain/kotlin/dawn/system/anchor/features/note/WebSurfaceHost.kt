package dawn.system.anchor.features.note

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import anchor.shared.generated.resources.Res
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.MetaStyle
import androidx.compose.runtime.Stable
import dawn.system.anchor.services.platform.Haptic
import dawn.system.anchor.services.platform.WebHostHandle
import dawn.system.anchor.services.platform.WebHostSurface
import dawn.system.anchor.services.platform.performHaptic
import dawn.system.anchor.services.platform.rememberWebHost
import dawn.system.anchor.services.platform.documentsDirPath
import dawn.system.anchor.services.platform.fileExists
import dawn.system.anchor.services.platform.fileSizeBytes
import dawn.system.anchor.services.platform.writeBytes
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import org.jetbrains.compose.resources.ExperimentalResourceApi

/** One web-backed artifact open inside the host. */
data class WebPane(val id: String, val kind: Kind, val title: String, val rev: Long = 0) {
    enum class Kind(val wire: String) {
        /** A BlockNote page, editing the same Y.Doc desktop is editing. */
        Page("page"),

        /** An agent-authored shard, sandboxed to an opaque origin. */
        Shard("shard"),

        /** A tldraw board, saved as the board artifact's snapshot. */
        Board("board"),
    }
}

/**
 * The companion's **one** web view, and everything web-backed inside it.
 *
 * Reimplementing a block editor and a Yjs client in Kotlin would be weeks of work for a
 * *second* implementation of the block model, to be kept in step with desktop forever.
 * Instead the companion carries the real editor as a self-contained document
 * (`aladin_react`'s `npm run build:embed`) and loads it from disk — served to WebKit under an
 * `http://localhost/` base, see `WebHostHandle.loadEmbedDocument` for why that is load-bearing.
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
 *
 * The value itself is the live view, once there is a document to load.
 */
@Stable
data class WebSurfaceHost(val handle: WebHostHandle, val restarts: Int)

/**
 * What the editor needs before it can join a document.
 *
 * **Plain values, not the services they came from.** The presenter is where a `TokenProvider`
 * becomes a token and an `ApiConfig` becomes a URL, so the shell state stays comparable and
 * assertable, and the UI layer never holds a service it could call.
 */
data class EditorSession(
    val token: String,
    val collabWsUrl: String,
    /** The board sync room server's origin — boards refuse to mount without it. */
    val boardSyncWsUrl: String,
    /**
     * The API origin. Only shards need it — a note is served entirely by the collab
     * socket — but it is read once at view creation, so it has to be here before the
     * first shard opens rather than when one does.
     */
    val apiBaseUrl: String,
    /** The name on the collaboration cursor; null falls back to a generic device name. */
    val userName: String?,
)

/**
 * Creates the one web view, at a scope that outlives the pager.
 *
 * Two things happen here, both once: unpack the bundle to disk (WKWebView needs a real path,
 * and Compose resources live inside the app bundle), then build the view. The result is a
 * handle the shell's web page attaches — never owns.
 *
 * **[enabled] defers the cost to the first note rather than the first launch.** A WebContent
 * process is ~187 MB, which someone who only ever reads PDFs should not pay. It is a latch,
 * not a switch: `documentPath` is never cleared once set, so closing the last note does not
 * tear the editor down and reopening one does not pay for it again.
 */
@OptIn(ExperimentalResourceApi::class)
@Composable
fun rememberWebSurfaceHost(
    session: EditorSession?,
    enabled: Boolean,
    onOpenArtifact: (String, Int?) -> Unit = { _, _ -> },
    onAskAbout: (HostMessage.AskAbout) -> Unit = {},
    onHaptic: (Haptic) -> Unit = ::performHaptic,
): WebSurfaceHost? {
    var documentPath by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(enabled) {
        if (enabled && documentPath == null) {
            documentPath = runCatching { unpackEditorDocument { Res.readBytes(EDITOR_RESOURCE) } }
                .getOrNull()
        }
    }

    // A dead web content process leaves a blank pane and nothing else; surface it. The
    // command is re-evaluated on the reload that follows, which is why it has to describe
    // state rather than a sequence of operations.
    var restarts by remember { mutableStateOf(0) }

    val path = documentPath ?: return null
    // No session, no view: the editor authenticates to the collab server with the injected
    // token alone, so building it without one would produce a pane that never syncs.
    val editor = session ?: return null

    // The bootstrap is read once, when the view is built. A token refresh mid-session
    // therefore does not reach the collab connection — a real gap, but a far smaller one
    // than making the token part of this view's identity, which would close every open
    // editor the moment it rotated.
    val handle = rememberWebHost(
        filePath = path,
        readAccessDirPath = editorDirPath(),
        bootstrapJs = embedBootstrap(
            token = editor.token,
            collabWsUrl = editor.collabWsUrl,
            boardSyncWsUrl = editor.boardSyncWsUrl,
            userName = editor.userName ?: DEFAULT_USER_NAME,
            apiBaseUrl = editor.apiBaseUrl,
        ),
        onMessage = { raw ->
            when (val message = parseHostMessage(raw)) {
                is HostMessage.OpenArtifact -> onOpenArtifact(message.id, message.page)
                is HostMessage.AskAbout -> onAskAbout(message)
                is HostMessage.Haptic -> onHaptic(message.kind)
                is HostMessage.WebError -> println("anchor embed error: ${message.message}")
                HostMessage.Ready, null -> Unit
            }
        },
        onContentProcessTerminated = {
            restarts++
            println("anchor embed: WEB CONTENT PROCESS TERMINATED (restart #$restarts)")
        },
    )

    return remember(handle, restarts) { WebSurfaceHost(handle, restarts) }
}

/**
 * The pager's web page: attaches the shared view and tells it what to show.
 *
 * `imePadding` here and NOT on the shell's columns — the editor is the only surface that
 * takes text, so it is the only one that should give way to the keyboard. WebKit then
 * scrolls the caret within the smaller viewport, which is one thing moving instead of two.
 *
 * …except for a **board**. Shrinking a tldraw canvas re-measures its viewport, and the
 * camera visibly jumps every time a task or card takes the keyboard; the plane keeps its
 * size and the editing object stays where the finger put it (see [givesWayToKeyboard]).
 */
@Composable
fun WebSurfacePage(
    host: WebSurfaceHost,
    panes: List<WebPane>,
    activeId: String?,
    bottomInset: Dp = 0.dp,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors
    val activeKind = panes.firstOrNull { it.id == activeId }?.kind

    Box(modifier.fillMaxSize().background(c.bg)) {
        WebHostSurface(
            handle = host.handle,
            command = syncCommand(panes, activeId, bottomInset.value.toInt()),
            modifier = Modifier.fillMaxSize().let { if (givesWayToKeyboard(activeKind)) it.imePadding() else it },
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

/**
 * Whether the web view shrinks above the keyboard for this kind of pane. Text surfaces do
 * (the caret must stay visible); a board does not (a resized canvas jumps its camera).
 */
internal fun givesWayToKeyboard(kind: WebPane.Kind?): Boolean = kind != WebPane.Kind.Board

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

/** What the embedded page can ask of the shell, web → native. */
sealed interface HostMessage {
    /** The page's React root mounted and drained the queued state. */
    data object Ready : HostMessage

    /**
     * The board's selection bar: "Open in folder" / "Open source" — navigate to this
     * artifact; [page] is the wormhole's landing spot (a cite), null for a plain open.
     */
    data class OpenArtifact(val id: String, val page: Int? = null) : HostMessage

    /**
     * The board's selection bar: "Ask about this" — open the copilot with the object as
     * context. [artifactId] is null for objects that are not windows onto an artifact
     * (a task, a card); [title] is what the bar showed.
     */
    data class AskAbout(val artifactId: String?, val title: String) : HostMessage

    /** A tap in the page changed something (tool, insert, flip) — play the device's tick. */
    data class Haptic(val kind: dawn.system.anchor.services.platform.Haptic) : HostMessage

    /** A JS error the page trapped — diagnostics only; the page shows its own notice. */
    data class WebError(val message: String) : HostMessage
}

private val hostMessageJson = Json { ignoreUnknownKeys = true }

/**
 * Parses one bridge message. The page always posts a JSON STRING (an object's native
 * toString is not parseable), and unknown or malformed messages are ignored — a newer
 * bundle must not crash an older shell.
 */
internal fun parseHostMessage(raw: String): HostMessage? = runCatching {
    val obj = hostMessageJson.parseToJsonElement(raw).jsonObject
    when (obj["type"]?.jsonPrimitive?.content) {
        "ready" -> HostMessage.Ready
        "openArtifact" -> obj["id"]?.jsonPrimitive?.content
            ?.takeIf { it.isNotBlank() }
            ?.let { id ->
                val page = obj["page"]
                    ?.takeIf { it !is JsonNull }
                    ?.jsonPrimitive?.content?.toIntOrNull()
                    ?.takeIf { it >= 1 }
                HostMessage.OpenArtifact(id, page)
            }
        "askAbout" -> obj["title"]?.jsonPrimitive?.content
            ?.takeIf { it.isNotBlank() }
            ?.let { title ->
                val artifactId = obj["artifactId"]
                    ?.takeIf { it !is JsonNull }
                    ?.jsonPrimitive?.content
                    ?.takeIf { it.isNotBlank() }
                HostMessage.AskAbout(artifactId, title)
            }
        "haptic" -> when (obj["kind"]?.jsonPrimitive?.content) {
            "light" -> HostMessage.Haptic(Haptic.Light)
            "medium" -> HostMessage.Haptic(Haptic.Medium)
            "select" -> HostMessage.Haptic(Haptic.Select)
            else -> null
        }
        "error" -> HostMessage.WebError(obj["message"]?.jsonPrimitive?.content ?: "unknown")
        else -> null
    }
}.getOrNull()

/** Session configuration, as one global set before the page's own scripts run. */
internal fun embedBootstrap(
    token: String,
    collabWsUrl: String,
    boardSyncWsUrl: String,
    userName: String,
    apiBaseUrl: String,
): String = "window.__ALADIN_EMBED__={" +
    "token:${jsString(token)}," +
    "collabWsUrl:${jsString(collabWsUrl)}," +
    "boardSyncWsUrl:${jsString(boardSyncWsUrl)}," +
    "apiBaseUrl:${jsString(apiBaseUrl)}," +
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
internal fun syncCommand(panes: List<WebPane>, activeId: String?, bottomInset: Int = 0): String {
    val list = panes.joinToString(",") { pane ->
        "{id:${jsString(pane.id)},kind:${jsString(pane.kind.wire)},title:${jsString(pane.title)}," +
            "rev:${pane.rev}}"
    }
    val active = activeId?.takeIf { id -> panes.any { it.id == id } }
    // `insets.bottom`: native chrome floating over the page's bottom edge — the shell's
    // dock while the sidebar is collapsed. A surface with its own floating chrome (the
    // board's tool dock) lifts it by this much, the nav handoff's rule for the Graph's zoom
    // control ("bottom 24px → 110px while the dock shows"). Part of the same declarative
    // state, so a re-send after a web process death restores it too.
    return "window.__aladinHost.sync({panes:[$list],active:${active?.let(::jsString) ?: "null"}," +
        "insets:{bottom:${bottomInset.coerceAtLeast(0)}}});"
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
