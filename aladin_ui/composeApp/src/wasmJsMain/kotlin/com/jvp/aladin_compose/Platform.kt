package com.jvp.aladin_compose

import kotlin.js.ExperimentalWasmJsInterop
import kotlinx.browser.window

interface Platform {
    val name: String
}

class WasmPlatform : Platform {
    override val name: String = "Web with Kotlin/Wasm"
}

fun getPlatform(): Platform = WasmPlatform()

fun openUrl(url: String) {
    window.open(url, "_blank")
}

@OptIn(ExperimentalWasmJsInterop::class)
fun copyTextToClipboard(text: String) {
    window.navigator.clipboard.writeText(text)
}
