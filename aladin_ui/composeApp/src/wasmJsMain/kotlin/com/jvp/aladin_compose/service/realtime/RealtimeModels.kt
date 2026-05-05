package com.jvp.aladin_compose.service.realtime

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject

@Serializable
data class SubscriptionKey(
    val stream: String,
    val resourceKind: String = "*",
    val resourceId: String = "*",
    val qualifiers: Map<String, String> = emptyMap(),
)

@Serializable
data class AppEvent(
    val eventId: String,
    val type: String,
    val subscriptionKey: SubscriptionKey,
    val payload: JsonElement = JsonObject(emptyMap()),
    val occurredAt: String? = null,
)

@Serializable
data class RealtimeSubscribeMessage(
    val type: String = "subscribe",
    val subscriptions: List<SubscriptionKey>,
)

@Serializable
data class RealtimeServerMessage(
    val type: String,
    val event: AppEvent? = null,
    val message: String? = null,
)
