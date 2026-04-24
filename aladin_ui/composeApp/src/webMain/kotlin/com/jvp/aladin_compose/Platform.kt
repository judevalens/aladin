package com.jvp.aladin_compose

interface Platform {
    val name: String
}

expect fun getPlatform(): Platform
expect fun openUrl(url: String)