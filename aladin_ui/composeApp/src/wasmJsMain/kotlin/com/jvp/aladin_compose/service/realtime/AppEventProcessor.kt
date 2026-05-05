package com.jvp.aladin_compose.service.realtime

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.launch

interface AppEventSource {
    val events: SharedFlow<AppEvent>
    fun start()
    fun stop()
}

interface AppEventListener {
    fun canHandle(subscriptionKey: SubscriptionKey, event: AppEvent): Boolean
    suspend fun handle(subscriptionKey: SubscriptionKey, event: AppEvent)
}

class AppEventProcessor(
    private val source: AppEventSource,
    private val listeners: Set<AppEventListener>,
    private val scope: CoroutineScope,
) {
    private val seenEventIds = linkedSetOf<String>()

    fun start() {
        source.start()
        scope.launch {
            source.events.collect { event ->
                if (!rememberEvent(event.eventId)) return@collect
                listeners
                    .filter { it.canHandle(event.subscriptionKey, event) }
                    .forEach { it.handle(event.subscriptionKey, event) }
            }
        }
    }

    fun stop() {
        source.stop()
    }

    private fun rememberEvent(eventId: String): Boolean {
        if (eventId in seenEventIds) return false
        seenEventIds += eventId
        if (seenEventIds.size > 512) {
            val first = seenEventIds.first()
            seenEventIds.remove(first)
        }
        return true
    }
}
