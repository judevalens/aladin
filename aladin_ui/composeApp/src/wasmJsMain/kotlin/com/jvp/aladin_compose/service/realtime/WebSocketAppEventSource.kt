package com.jvp.aladin_compose.service.realtime

import com.jvp.aladin_compose.api.BASE_URL
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.serialization.json.Json

private val realtimeJson = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
}

@OptIn(ExperimentalWasmJsInterop::class)
external interface BrowserRealtimeSocket : JsAny {
    fun send(text: String)
    fun close()
}

@OptIn(ExperimentalWasmJsInterop::class)
@JsFun(
    """
    (url, onOpen, onMessage, onClose, onError) => {
      const socket = new WebSocket(url);
      socket.onopen = () => onOpen();
      socket.onmessage = (event) => onMessage(String(event.data));
      socket.onclose = () => onClose();
      socket.onerror = () => onError();
      return {
        send: (text) => socket.send(text),
        close: () => socket.close()
      };
    }
    """,
)
external fun createBrowserRealtimeSocket(
    url: String,
    onOpen: () -> Unit,
    onMessage: (String) -> Unit,
    onClose: () -> Unit,
    onError: () -> Unit,
): BrowserRealtimeSocket

class WebSocketAppEventSource(
    private val subscriptions: List<SubscriptionKey> =
        listOf(
            SubscriptionKey(
                stream = "workspace",
                resourceKind = "*",
                resourceId = "*",
            ),
        ),
) : AppEventSource {
    private val mutableEvents = MutableSharedFlow<AppEvent>(extraBufferCapacity = 64)
    private var socket: BrowserRealtimeSocket? = null

    override val events: SharedFlow<AppEvent> = mutableEvents.asSharedFlow()

    override fun start() {
        if (socket != null) return

        socket =
            createBrowserRealtimeSocket(
                url = websocketUrl(),
                onOpen = {
                    socket?.send(
                        realtimeJson.encodeToString(
                            RealtimeSubscribeMessage(
                                subscriptions = subscriptions,
                            ),
                        ),
                    )
                },
                onMessage = { raw ->
                    val message =
                        runCatching {
                            realtimeJson.decodeFromString<RealtimeServerMessage>(raw)
                        }.getOrNull()
                    val event = message?.event ?: return@createBrowserRealtimeSocket
                    mutableEvents.tryEmit(event)
                },
                onClose = {
                    socket = null
                },
                onError = {
                    socket = null
                },
            )
    }

    override fun stop() {
        socket?.close()
        socket = null
    }

    private fun websocketUrl(): String {
        val base =
            when {
                BASE_URL.startsWith("https://") -> "wss://" + BASE_URL.removePrefix("https://")
                BASE_URL.startsWith("http://") -> "ws://" + BASE_URL.removePrefix("http://")
                else -> BASE_URL
            }
        return "$base/api/events/ws"
    }
}
