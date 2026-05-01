package com.jvp.aladin_compose.service.web

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
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
    val markdown: String
}

@OptIn(ExperimentalWasmJsInterop::class)
external fun createBridge(jsEventHandler: (JsEvent) -> Unit): WebBridge

external interface WebBridge : JsAny {
    fun mount(root: HTMLElement)

    fun unmount(root: HTMLElement)
}

@OptIn(ExperimentalComposeUiApi::class, ExperimentalWasmJsInterop::class)
@Composable
fun WebWidget(modifier: Modifier = Modifier) {
    val bridge = remember {
        createBridge { event ->
            when (event.type) {
                "documentUpdated" -> {
                    val markdown = event.payload
                        ?.unsafeCast<DocumentUpdatedPayload>()
                        ?.markdown
                    println("\nReceived documentUpdated event:\n $markdown")

                }
            }
            println("Received event: $event")
        }
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
        update = { element -> bridge.mount(element) },
        onRelease = { element -> bridge.unmount(element) },
    )
}
