package dawn.system.anchor.services.sync

import dawn.system.anchor.services.network.ApiConfig
import io.ktor.client.HttpClient
import io.ktor.client.plugins.websocket.webSocket
import io.ktor.websocket.Frame as WsFrame
import io.ktor.websocket.readText
import kotlinx.coroutines.channels.consumeEach
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * The live half of the sync spine, and the reason the companion is not a polling client.
 *
 * The server's CDC drain publishes every committed frame on the workspace realtime stream
 * as event kind `*.frame`. This subscribes to that stream and applies each frame through
 * **the same engine the puller uses**, so a change lands as soon as it is committed rather
 * than whenever a timer next fires.
 *
 * Two rules, both mirroring `src-tauri/src/sync/live.rs`, and both load-bearing:
 *
 *  1. **Live never advances the cursor.** It is best-effort — it may drop, duplicate, or
 *     reorder — so the cursor stays the puller's alone. Advancing it here would let a
 *     dropped frame be skipped forever.
 *  2. **Pull on connect.** The socket only carries what happens *after* it opens, so the
 *     gap since the last pull is closed by pulling once the subscription is live.
 *
 * The per-entity seq guard in the store makes the overlap between the two safe: a frame
 * that arrives on both paths applies once, and a stale one is ignored whichever path it
 * came from.
 */
class SyncLive(
    private val client: HttpClient,
    private val config: ApiConfig,
    private val engine: SyncEngine,
    private val json: Json,
) {
    /**
     * Connects, subscribes, and applies frames until the socket closes or [connect] is
     * cancelled. Returns normally on a clean close so the caller can decide to retry;
     * throws on a transport failure.
     *
     * [onSubscribed] fires once the server confirms the subscription — the caller uses it
     * to run the connect-time pull.
     */
    suspend fun connect(onSubscribed: suspend () -> Unit, onApplied: suspend (Int) -> Unit = {}) {
        client.webSocket(config.eventsWsUrl()) {
            send(WsFrame.Text(json.encodeToString(SUBSCRIBE)))

            incoming.consumeEach { frame ->
                if (frame !is WsFrame.Text) return@consumeEach
                val message = runCatching {
                    json.decodeFromString<RealtimeServerMessage>(frame.readText())
                }.getOrNull() ?: return@consumeEach

                when (message.type) {
                    "subscribed" -> onSubscribed()
                    else -> message.event
                        ?.takeIf { it.type.endsWith(FRAME_SUFFIX) }
                        ?.payload
                        ?.let { payload -> onApplied(engine.applyFrame(payload)) }
                }
            }
        }
    }

    private companion object {
        /** The one live event kind the server publishes for data frames. */
        const val FRAME_SUFFIX = ".frame"

        val SUBSCRIBE = SubscribeMessage(
            subscriptions = listOf(
                SubscriptionKey(
                    eventKind = "*$FRAME_SUFFIX",
                    stream = "workspace",
                    resourceKind = "*",
                    resourceId = "*",
                ),
            ),
        )
    }
}

@Serializable
internal data class RealtimeServerMessage(
    val type: String,
    val event: RealtimeEvent? = null,
    val message: String? = null,
)

@Serializable
internal data class RealtimeEvent(
    val eventId: String = "",
    val type: String = "",
    /** A sync [Frame] — the same shape the pull endpoint returns. */
    val payload: Frame? = null,
    val occurredAt: String = "",
)
