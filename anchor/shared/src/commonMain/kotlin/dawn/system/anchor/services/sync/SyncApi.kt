package dawn.system.anchor.services.sync

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.get
import io.ktor.client.request.parameter

/**
 * The pull half of the sync transport: frames with `xid > since`, plus the new horizon.
 *
 * An interface so the puller's rules can be tested against scripted responses without a
 * server — the Rust suite this mirrors does exactly that, and those tests are the
 * acceptance criteria for this port.
 */
interface SyncApi {
    suspend fun pull(since: Long): SyncPullResult
}

internal class KtorSyncApi(private val client: HttpClient) : SyncApi {
    override suspend fun pull(since: Long): SyncPullResult =
        client.get("/api/sync/pull") { parameter("since", since.toString()) }.body()
}
