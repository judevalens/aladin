package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.WebElementView
import kotlinx.browser.document
import org.w3c.dom.HTMLDivElement
import org.w3c.dom.HTMLElement

@JsName("AladinArtifactSpa")
external object AladinArtifactSpa {
    fun mount(element: HTMLElement, title: String, kind: String)
    fun unmount(element: HTMLElement)
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun ArtifactWebSurface(
    modifier: Modifier,
    title: String,
    kind: String,
) {
    WebElementView(
        modifier = modifier,
        factory = {
            (document.createElement("div") as HTMLDivElement).apply {
                className = "aladin-artifact-spa-host"
                style.width = "100%"
                style.height = "100%"
                style.overflowY = "auto"
                style.overflowX = "hidden"
                AladinArtifactSpa.mount(this, title, kind)
            }
        },
        update = { element ->
            AladinArtifactSpa.mount(element, title, kind)
        },
        onRelease = { element ->
            AladinArtifactSpa.unmount(element)
        },
    )
}
