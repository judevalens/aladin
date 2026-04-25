package com.jvp.aladin_compose

import androidx.compose.runtime.Composable
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.WebElementView
import kotlinx.browser.document
import org.w3c.dom.HTMLIFrameElement

@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun WebView(
    modifier: Modifier,
    url: String?,
    srcdoc: String?,
    sandbox: String?,
) {
    WebElementView(
        modifier = modifier,
        factory = {
            (document.createElement("iframe") as HTMLIFrameElement).apply {
                style.border = "0"
                style.width = "100%"
                style.height = "100%"
                url?.let { src = it }
                srcdoc?.let { this.srcdoc = it }
                sandbox?.let { setAttribute("sandbox", it) }
            }
        },
        update = { iframe ->
            url?.let { iframe.src = it }
            srcdoc?.let { iframe.srcdoc = it }
        }
    )
}
