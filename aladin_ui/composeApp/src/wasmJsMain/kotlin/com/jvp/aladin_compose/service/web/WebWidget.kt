package com.jvp.aladin_compose.service.web

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.WebElementView
import kotlinx.browser.document
import org.w3c.dom.HTMLDivElement
import org.w3c.dom.HTMLElement

external interface JsEvent : JsAny {
    val type: String
    val payload: JsAny?
}

external interface DocumentUpdatedPayload : JsAny {
    val pageId: String
    val markdown: String
}

external interface FileUploadRequestedPayload : JsAny {
    val pageId: String
    val requestId: String
    val filename: String
    val contentType: String?
    val base64Data: String
}

@OptIn(ExperimentalWasmJsInterop::class)
external fun createBridge(jsEventHandler: (JsEvent) -> Unit): WebBridge

external interface WebBridge : JsAny {
    fun mount(root: HTMLElement, pageId: String, initialMarkdown: String, isEditable: Boolean)

    fun notifyFileUploadCompleted(pageId: String, requestId: String, url: String)

    fun notifyFileUploadFailed(pageId: String, requestId: String, message: String)

    fun unmount(root: HTMLElement)
}

interface PageEditorBridge {
    fun register(pageId: String, bridge: WebBridge)

    fun unregister(pageId: String, bridge: WebBridge)

    fun notifyFileUploadCompleted(pageId: String, requestId: String, url: String)

    fun notifyFileUploadFailed(pageId: String, requestId: String, message: String)
}

class PageEditorBridgeImpl : PageEditorBridge {
    private val bridgesByPageId = mutableMapOf<String, WebBridge>()

    override fun register(pageId: String, bridge: WebBridge) {
        bridgesByPageId[pageId] = bridge
    }

    override fun unregister(pageId: String, bridge: WebBridge) {
        if (bridgesByPageId[pageId] == bridge) {
            bridgesByPageId.remove(pageId)
        }
    }

    override fun notifyFileUploadCompleted(pageId: String, requestId: String, url: String) {
        bridgesByPageId[pageId]?.notifyFileUploadCompleted(pageId, requestId, url)
    }

    override fun notifyFileUploadFailed(pageId: String, requestId: String, message: String) {
        bridgesByPageId[pageId]?.notifyFileUploadFailed(pageId, requestId, message)
    }
}

@OptIn(ExperimentalComposeUiApi::class, ExperimentalWasmJsInterop::class)
@Composable
fun WebWidget(
    pageId: String,
    initialMarkdown: String,
    isEditable: Boolean,
    onDocumentUpdated: (pageId: String, markdown: String) -> Unit,
    onFileUploadRequested:
        (
            pageId: String,
            requestId: String,
            filename: String,
            contentType: String?,
            base64Data: String,
        ) -> Unit,
    pageEditorBridge: PageEditorBridge,
    modifier: Modifier = Modifier,
) {
    val currentOnDocumentUpdated = rememberUpdatedState(onDocumentUpdated)
    val currentOnFileUploadRequested = rememberUpdatedState(onFileUploadRequested)
    val bridge =
        remember(pageId) {
            createBridge { event ->
                when (event.type) {
                    "documentUpdated" -> {
                        val payload = event.payload?.unsafeCast<DocumentUpdatedPayload>()
                        val updatedPageId = payload?.pageId ?: pageId
                        val markdown = payload?.markdown ?: return@createBridge
                        currentOnDocumentUpdated.value(updatedPageId, markdown)
                    }
                    "fileUploadRequested" -> {
                        val payload = event.payload?.unsafeCast<FileUploadRequestedPayload>()
                        val updatedPageId = payload?.pageId ?: pageId
                        val requestId = payload?.requestId ?: return@createBridge
                        val filename = payload.filename
                        val base64Data = payload.base64Data
                        currentOnFileUploadRequested.value(
                            updatedPageId,
                            requestId,
                            filename,
                            payload.contentType,
                            base64Data,
                        )
                    }
                }
            }
        }
    DisposableEffect(pageId, bridge, pageEditorBridge) {
        pageEditorBridge.register(pageId, bridge)
        onDispose { pageEditorBridge.unregister(pageId, bridge) }
    }
    WebElementView(
        modifier = modifier,
        factory = {
            (document.createElement("div") as HTMLDivElement).apply {
                className = "aladin-artifact-spa-host"
                style.width = "100%"
                style.height = "100%"
                style.overflowY = "auto"
                style.overflowX = "hidden"
            }
        },
        update = { element -> bridge.mount(element, pageId, initialMarkdown, isEditable) },
        onRelease = { element -> bridge.unmount(element) },
    )
}
