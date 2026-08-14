package dawn.system.anchor.services.sync

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonObject

/**
 * The sync wire model, mirroring `backend_v2/internal/service/sync_types.go`.
 *
 * **`seq` and `cursor` are uint64 encoded as JSON strings** (Go's `json:",string"`),
 * because they exceed the JavaScript safe-integer range and the same payload feeds a web
 * client. Decoding them as numbers would silently corrupt large values, so they go through
 * [StringLongSerializer] and never touch a numeric JSON field.
 */
@Serializable
data class FrameEntity(
    val entityKind: String,
    val entityId: String,
    @Serializable(with = StringLongSerializer::class) val seq: Long,
    val op: String,
    val data: JsonObject? = null,
)

@Serializable
data class Frame(
    val entities: List<FrameEntity> = emptyList(),
)

@Serializable
data class SyncPullResult(
    val frames: List<Frame> = emptyList(),
    @Serializable(with = StringLongSerializer::class) val cursor: Long,
    val mode: String,
) {
    /** A snapshot is authoritative and REPLACES the cache; a delta merges into it. */
    val isSnapshot: Boolean get() = mode == MODE_SNAPSHOT

    companion object {
        const val MODE_SNAPSHOT = "snapshot"
    }
}

/** The live socket's subscription key. */
@Serializable
data class SubscriptionKey(
    @SerialName("eventKind") val eventKind: String? = null,
    val stream: String,
    val resourceKind: String,
    val resourceId: String,
)

@Serializable
data class SubscribeMessage(
    val type: String = "subscribe",
    val subscriptions: List<SubscriptionKey>,
)

internal object StringLongSerializer : KSerializer<Long> {
    override val descriptor: SerialDescriptor =
        PrimitiveSerialDescriptor("StringLong", PrimitiveKind.STRING)

    override fun deserialize(decoder: Decoder): Long =
        decoder.decodeString().toLongOrNull() ?: 0L

    override fun serialize(encoder: Encoder, value: Long) =
        encoder.encodeString(value.toString())
}
