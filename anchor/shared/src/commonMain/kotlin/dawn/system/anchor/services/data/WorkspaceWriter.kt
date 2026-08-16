package dawn.system.anchor.services.data

import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.sync.StringLongSerializer
import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.delete
import io.ktor.client.request.patch
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.client.statement.HttpResponse
import io.ktor.http.ContentType
import io.ktor.http.contentType
import io.ktor.http.isSuccess
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonObjectBuilder
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.put

/**
 * Workspace writes — create, rename, delete — mirroring the Rust client's
 * `src-tauri/src/api/workspace_write.rs` and `db/repo/workspace.rs`.
 *
 * **Writes do not go into the sync spine; they go around it.** The client proxies the
 * mutation to the Go REST API, which commits it and hands back the committed row *with its
 * `seq`*. That result is then applied through the very same per-entity seq guard a frame
 * goes through, so the row appears at once and the live frame that follows a moment later
 * is a no-op rather than a second, conflicting write.
 *
 * That is not an optimistic patch, which is the thing this codebase forbids: nothing is
 * written locally until the server has committed it and said what the committed version is.
 * The alternative — write, then wait for the socket — makes every rename look broken for as
 * long as the round trip takes, and look permanently broken if the socket happens to be down.
 */
interface WorkspaceWriter {
    /** Creates a folder inside [parentId] (null = the section root). Returns its id. */
    suspend fun createFolder(parentId: String?, title: String): String

    /** Creates an empty BlockNote page inside [parentId]. Returns its artifact id. */
    suspend fun createPage(parentId: String?, title: String): String

    /** Renames [node], dispatching on its kind — the three tree kinds have three endpoints. */
    suspend fun rename(node: WorkspaceNode, title: String)

    /** Soft-deletes a node and its subtree. */
    suspend fun delete(node: WorkspaceNode)
}

internal class KtorWorkspaceWriter(
    private val client: HttpClient,
    private val nodes: NodeStore,
) : WorkspaceWriter {

    override suspend fun createFolder(parentId: String?, title: String): String =
        create(
            buildJsonObject {
                put("kind", "folder")
                put("title", title)
                putNullable("parentId", parentId)
            },
        )

    override suspend fun createPage(parentId: String?, title: String): String =
        create(
            buildJsonObject {
                put("kind", "artifact")
                put("title", title)
                putNullable("parentId", parentId)
                // A page's body belongs to the collaborative Y.Doc, never to this call —
                // the server creates it empty and the editor supplies the first content.
                put(
                    "artifact",
                    buildJsonObject {
                        put("type", "page")
                        put("content", "")
                    },
                )
            },
        )

    private suspend fun create(body: JsonObject): String {
        val result: CreateNodeResult = client
            .post("/api/browser/nodes") { contentType(ContentType.Application.Json); setBody(body) }
            .requireSuccess("create")
            .body()

        val node = result.node
        nodes.apply(
            kind = node.kind,
            id = node.id,
            seq = node.seq,
            op = OP_UPSERT,
            data = buildJsonObject {
                put("kind", node.kind)
                putNullable("parentId", node.parentId)
                put("position", node.position.toString())
                put("title", node.title)
                putNullable("type", result.artifact?.type)
                putNullable("sourceUrl", result.artifact?.sourceUrl)
            },
        )
        return node.id
    }

    override suspend fun rename(node: WorkspaceNode, title: String) {
        val path = when (node.kind) {
            // The folder endpoint is deliberately folder-only; a research folder renames
            // through its own, as it carries a sub-object the folder API knows nothing about.
            NodeKind.Folder -> "/api/folders/${node.id}"
            NodeKind.Research -> "/api/research/${node.id}"
            NodeKind.Artifact -> "/api/artifacts/${node.id}"
        }

        val renamed: RenamedNode = client
            .patch(path) {
                contentType(ContentType.Application.Json)
                setBody(buildJsonObject { put("title", title) })
            }
            .requireSuccess("rename")
            .body()

        // A rename DTO carries only the fields it changed, so applying it as-is through the
        // upsert — which replaces every column — would blank the node's parent and kind.
        // Merge onto what is already cached instead.
        val current = nodes.byId(node.id) ?: node
        nodes.apply(
            kind = current.kind.wire,
            id = node.id,
            seq = renamed.seq,
            op = OP_UPSERT,
            data = buildJsonObject {
                put("kind", current.kind.wire)
                putNullable("parentId", current.parentId)
                put("position", current.position.toString())
                put("title", renamed.title.ifBlank { title })
                putNullable("type", current.artifactType)
                putNullable("sourceUrl", current.sourceUrl)
                putNullable("summary", current.summary)
            },
        )
    }

    override suspend fun delete(node: WorkspaceNode) {
        val deleted: DeletedNode = client
            .delete("/api/browser/nodes/${node.id}")
            .requireSuccess("delete")
            .body()

        // Descendants heal via the frame that follows; the tombstone written here is the
        // one the user is looking at.
        nodes.apply(
            kind = node.kind.wire,
            id = deleted.id,
            seq = deleted.seq,
            op = OP_DELETE,
            data = null,
        )
    }

    private fun HttpResponse.requireSuccess(action: String): HttpResponse = also {
        if (!status.isSuccess()) error("the server refused the $action (${status.value})")
    }

    private companion object {
        const val OP_UPSERT = "upsert"
        const val OP_DELETE = "delete"
    }
}

/** Emits an explicit JSON null rather than omitting the key, which the guard reads as "clear". */
private fun JsonObjectBuilder.putNullable(key: String, value: String?) {
    put(key, if (value == null) JsonNull else JsonPrimitive(value))
}

/** The committed light node a write returns (mirrors Go `service.BrowserNodeResponse`). */
@Serializable
private data class WrittenNode(
    val id: String,
    val parentId: String? = null,
    val kind: String,
    val title: String = "",
    val position: Long = 0,
    @Serializable(with = StringLongSerializer::class) val seq: Long,
)

/** The artifact body a create returns (mirrors Go `service.ArtifactResponse`). */
@Serializable
private data class WrittenArtifact(
    val id: String,
    @SerialName("type") val type: String? = null,
    val title: String = "",
    val sourceUrl: String? = null,
    @Serializable(with = StringLongSerializer::class) val seq: Long = 0,
)

@Serializable
private data class CreateNodeResult(
    val node: WrittenNode,
    val artifact: WrittenArtifact? = null,
)

@Serializable
private data class RenamedNode(
    val id: String = "",
    val title: String = "",
    // Defaulted rather than required: not every kind's PATCH answers with a version. A
    // zero fails the seq guard and writes nothing, so the change simply arrives on the
    // live frame instead — slower, never wrong.
    @Serializable(with = StringLongSerializer::class) val seq: Long = 0,
)

@Serializable
private data class DeletedNode(
    val id: String = "",
    @Serializable(with = StringLongSerializer::class) val seq: Long = 0,
)
