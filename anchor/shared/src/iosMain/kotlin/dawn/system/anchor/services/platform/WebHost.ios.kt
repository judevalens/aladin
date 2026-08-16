package dawn.system.anchor.services.platform

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.Stable
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.UIKitView
import kotlinx.cinterop.ExperimentalForeignApi
import kotlinx.cinterop.readValue
import platform.CoreGraphics.CGRectZero
import platform.Foundation.NSURL
import platform.WebKit.WKNavigation
import platform.WebKit.WKNavigationDelegateProtocol
import platform.WebKit.WKScriptMessage
import platform.WebKit.WKScriptMessageHandlerProtocol
import platform.WebKit.WKUserContentController
import platform.WebKit.WKUserScript
import platform.WebKit.WKUserScriptInjectionTime
import platform.WebKit.WKWebView
import platform.WebKit.WKWebViewConfiguration
import platform.darwin.NSObject

/**
 * The live `WKWebView`, plus the two delegates WebKit holds weakly and the small amount of
 * state that has to outlive any single attachment.
 *
 * Detaching a `WKWebView` from the view hierarchy does not destroy it: the JS heap, the DOM
 * and the open documents survive. WebKit will throttle rendering and may suspend the
 * process while it is out of the hierarchy, so a long detachment can cost a reconnect — but
 * that path is the same one a web content process death already takes, and the command is
 * idempotent precisely so re-sending it is the whole recovery.
 */
@Stable
@OptIn(ExperimentalForeignApi::class)
actual class WebHostHandle internal constructor(
    filePath: String,
    readAccessDirPath: String,
    bootstrapJs: String,
    onMessage: (String) -> Unit,
    private val onTerminated: () -> Unit,
) {
    /** Bumped on every completed navigation; read by the surface to re-send its command. */
    internal var generation by mutableStateOf(0)
        private set

    /** The last `generation:command` actually evaluated, so re-attaching is not a re-send. */
    internal var applied: String = ""

    private val bridge = WebBridge(onMessage)

    private val navigation = object : NSObject(), WKNavigationDelegateProtocol {
        override fun webView(webView: WKWebView, didFinishNavigation: WKNavigation?) {
            generation++
        }

        override fun webViewWebContentProcessDidTerminate(webView: WKWebView) {
            onTerminated()
            webView.reload()
        }
    }

    internal val view: WKWebView = WKWebView(
        frame = CGRectZero.readValue(),
        configuration = WKWebViewConfiguration().apply {
            if (bootstrapJs.isNotEmpty()) {
                userContentController.addUserScript(
                    WKUserScript(
                        source = bootstrapJs,
                        injectionTime = WKUserScriptInjectionTime.WKUserScriptInjectionTimeAtDocumentStart,
                        forMainFrameOnly = true,
                    ),
                )
            }
            userContentController.addScriptMessageHandler(bridge, BRIDGE_NAME)
        },
    ).apply {
        setOpaque(false)
        navigationDelegate = navigation
        loadFileURL(
            URL = NSURL.fileURLWithPath(filePath),
            allowingReadAccessToURL = NSURL.fileURLWithPath(readAccessDirPath, isDirectory = true),
        )
    }

    internal fun evaluate(command: String) {
        view.evaluateJavaScript(command, null)
    }
}

@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun rememberWebHost(
    filePath: String,
    readAccessDirPath: String,
    bootstrapJs: String,
    onMessage: (String) -> Unit,
    onContentProcessTerminated: () -> Unit,
): WebHostHandle {
    val incoming by rememberUpdatedState(onMessage)
    val terminated by rememberUpdatedState(onContentProcessTerminated)

    // Keyed on the document alone. NOT on the bootstrap: that carries the bearer token, and
    // making it part of this object's identity would tear down every open editor the first
    // time a token refreshed.
    return remember(filePath) {
        WebHostHandle(
            filePath = filePath,
            readAccessDirPath = readAccessDirPath,
            bootstrapJs = bootstrapJs,
            onMessage = { incoming(it) },
            onTerminated = { terminated() },
        )
    }
}

@OptIn(ExperimentalForeignApi::class)
@Composable
actual fun WebHostSurface(handle: WebHostHandle, command: String, modifier: Modifier) {
    UIKitView(
        // Hands back the view the handle already owns rather than building one. Leaving
        // composition removes it from its superview; the handle keeps it alive, so coming
        // back re-attaches the same live page.
        factory = { handle.view },
        update = {
            val stamp = "${handle.generation}:$command"
            if (command.isNotEmpty() && stamp != handle.applied) {
                handle.applied = stamp
                // Re-sent on every navigation, not only when the state changes. The first
                // command would otherwise race the initial load: evaluating into a document
                // that is still being replaced runs in the outgoing context and is thrown
                // away with it — a host that never opens anything.
                handle.evaluate(command)
            }
        },
        modifier = modifier,
    )
}

/** The message-handler name the page posts to: `webkit.messageHandlers.anchor`. */
private const val BRIDGE_NAME = "anchor"

/** Web → native. One handler for the whole host; the payload names its own type. */
private class WebBridge(
    private val onMessage: (String) -> Unit,
) : NSObject(), WKScriptMessageHandlerProtocol {

    override fun userContentController(
        userContentController: WKUserContentController,
        didReceiveScriptMessage: WKScriptMessage,
    ) {
        onMessage(didReceiveScriptMessage.body.toString())
    }
}
