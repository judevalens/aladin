package com.jvp.aladin_compose.repo

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.FileUploadRequest
import com.jvp.aladin_compose.api.PageDocumentRecord
import com.jvp.aladin_compose.api.UploadedFileRecord
import com.jvp.aladin_compose.api.UserArtifact
import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.repo.doa.ArtifactDoa
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.emitAll
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.runningFold
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull

interface ArtifactRepository {
    fun artifact(id: String): Flow<Artifact>

    fun artifacts(ids: List<String>): Flow<List<Artifact>>

    fun observeArtifactBreadcrumbs(id: String?): Flow<List<BreadcrumbItem>>

    fun registerLocalArtifact(artifact: Artifact)

    fun linkContent(artifact: Artifact): Flow<LinkArtifactContent>

    fun voiceContent(artifact: Artifact): Flow<VoiceArtifactContent>

    fun artifactInsight(artifact: Artifact): Flow<ArtifactInsight>

    suspend fun page(id: String): Flow<PageDocument>

    suspend fun savePage(id: String, markdown: String, revision: Long): PageDocument

    suspend fun renameArtifact(id: String, title: String): Artifact

    suspend fun uploadFile(filename: String, bytes: ByteArray, contentType: String?): UploadedFile

    fun artifactResourceUrl(id: String): String
}

data class PageDocument(
    val id: String,
    val title: String,
    val content: String,
    val revision: Long,
    val updatedAt: String,
)

data class UploadedFile(val id: String, val url: String, val uploadedAt: String)

data class LinkArtifactContent(
    val artifactId: String,
    val sourceUrl: String,
    val displayDomain: String,
    val rawExcerpt: String,
    val aiSummary: String,
    val userNotes: String,
    val statusLabel: String,
)

data class VoiceArtifactContent(
    val artifactId: String,
    val resourceUrl: String?,
    val durationLabel: String,
    val recordedAtLabel: String,
    val transcript: String,
    val userNotes: String,
    val statusLabel: String,
)

data class ArtifactInsight(
    val artifactId: String,
    val summary: String?,
    val keyPoints: List<String>,
    val relatedItems: List<RelatedArtifact>,
    val entities: List<String>,
    val metadata: List<ArtifactMetadataItem>,
)

data class RelatedArtifact(
    val id: String,
    val title: String,
    val kind: ArtifactKind,
)

data class ArtifactMetadataItem(val label: String, val value: String)

class ArtifactRepositoryImpl(val inMemoryArtifactDoa: ArtifactDoa) : ArtifactRepository {

    private val localArtifactsById = mutableMapOf<String, Artifact>()
    val artifactFlow: MutableSharedFlow<Artifact> = MutableSharedFlow()
    val pageFLow: MutableSharedFlow<PageDocument> = MutableSharedFlow()

    override fun artifact(id: String): Flow<Artifact> = flow {
        val artifact = localArtifactsById[id] ?: inMemoryArtifactDoa.getArtifact(id)?.let(::userArtifactToArtifact)
        if (artifact != null) {
            emit(artifact)
            emitAll(artifactFlow.filter { it.id == id })
        } else {
            throw IllegalStateException("Artifact not found")
        }
    }

    override fun artifacts(ids: List<String>): Flow<List<Artifact>> {
        if (ids.isEmpty()) return flowOf(emptyList())
        val idSet = ids.toSet()

        return flow {
            val initialArtifacts =
                ids.mapNotNull {
                        localArtifactsById[it]
                            ?: inMemoryArtifactDoa.getArtifact(it)?.let(::userArtifactToArtifact)
                    }
                    .associateBy { it.id }

            val updateFlow =
                artifactFlow
                    .filter { it.id in idSet }
                    .runningFold(initialArtifacts) { oldArtifacts, updatedArtifact ->
                        oldArtifacts + (updatedArtifact.id to updatedArtifact)
                    }
                    .map { artifacts -> ids.mapNotNull { artifacts[it] } }
            emitAll(updateFlow)
        }
    }

    override fun observeArtifactBreadcrumbs(id: String?): Flow<List<BreadcrumbItem>> {
        if (id == null) return flowOf(listOf(BreadcrumbItem(id = null, label = "Folders")))

        return flow {
            val initialArtifact =
                localArtifactsById[id] ?: inMemoryArtifactDoa.getArtifact(id)?.let(::userArtifactToArtifact)
                    ?: throw IllegalStateException("Artifact not found")
            emit(artifactBreadcrumbs(initialArtifact))
            val updateFlow =
                artifactFlow
                    .filter { it.id == id }
                    .map { artifact -> artifactBreadcrumbs(artifact) }
            emitAll(updateFlow)
        }
    }

    override fun registerLocalArtifact(artifact: Artifact) {
        localArtifactsById[artifact.id] = artifact
    }

    override fun linkContent(artifact: Artifact): Flow<LinkArtifactContent> =
        flowOf(
            LinkArtifactContent(
                artifactId = artifact.id,
                sourceUrl = artifact.sourceUrl ?: "https://example.com/source/${artifact.id}",
                displayDomain = displayDomain(artifact.sourceUrl),
                rawExcerpt =
                    artifact.summary
                        ?: "Raw captured source text will appear here once link ingestion is wired. For now this stub keeps the consumption layout realistic without backend enrichment.",
                aiSummary =
                    "This saved source appears relevant to the current workspace because it adds external context, evidence, or a reusable reference point.",
                userNotes =
                    "User notes will live with the link artifact. This keeps the source object independent while still letting pages embed it later.",
                statusLabel = "Stub summary",
            )
        )

    override fun voiceContent(artifact: Artifact): Flow<VoiceArtifactContent> =
        flowOf(
            VoiceArtifactContent(
                artifactId = artifact.id,
                resourceUrl = artifact.resourceUrl,
                durationLabel = "04:12",
                recordedAtLabel = artifact.updatedLabel,
                transcript =
                    "Transcript will appear here after voice transcription is wired. This stub is shaped like the final consumption surface: audio remains the source of truth, and text becomes searchable context.",
                userNotes =
                    "User notes can capture the interpretation or follow-up thought that belongs with this recording.",
                statusLabel = "Transcript pending",
            )
        )

    override fun artifactInsight(artifact: Artifact): Flow<ArtifactInsight> =
        flowOf(
            ArtifactInsight(
                artifactId = artifact.id,
                summary =
                    when (artifact.kind) {
                        ArtifactKind.Note ->
                            "This page is being interpreted as a working document. Future enrichment will summarize the draft and connect it to source artifacts."
                        ArtifactKind.Link ->
                            "This link is treated as an external source. Future ingestion will extract title, author, key claims, and workspace relevance."
                        ArtifactKind.Voice ->
                            "Voice enrichment will summarize transcript, key moments, and detected action items after recording support lands."
                        ArtifactKind.File ->
                            "File enrichment will expose document metadata, preview text, and related workspace context."
                    },
                keyPoints =
                    when (artifact.kind) {
                        ArtifactKind.Link ->
                            listOf(
                                "Source can be opened independently from the workspace.",
                                "AI summary and user notes stay attached to the link artifact.",
                                "Pages can later embed this link by reference.",
                            )
                        ArtifactKind.Note ->
                            listOf(
                                "User-authored markdown remains the source of truth.",
                                "System context stays outside the editable page body.",
                            )
                        else -> emptyList()
                    },
                relatedItems =
                    listOf(
                        RelatedArtifact(
                            id = "related-${artifact.id}-page",
                            title = "Workspace context note",
                            kind = ArtifactKind.Note,
                        ),
                        RelatedArtifact(
                            id = "related-${artifact.id}-source",
                            title = "Source mentioned nearby",
                            kind = ArtifactKind.Link,
                        ),
                    ),
                entities =
                    when (artifact.kind) {
                        ArtifactKind.Link -> listOf("source", "reference", "external context")
                        ArtifactKind.Note -> listOf("working note", "draft", "workspace")
                        else -> emptyList()
                    },
                metadata =
                    listOf(
                        ArtifactMetadataItem("Type", artifact.kind.name),
                        ArtifactMetadataItem("Updated", artifact.updatedLabel),
                    ),
            )
        )

    override suspend fun page(id: String): Flow<PageDocument> = flow {
        val pageDocument = inMemoryArtifactDoa.getPage(id)?.let(::toPageDocument)
        if (pageDocument != null) {
            emit(pageDocument)
            emitAll(pageFLow.filter { it.id == id })
        } else {
            throw IllegalStateException("Page not found")
        }
    }

    override suspend fun savePage(id: String, markdown: String, revision: Long): PageDocument =
        toPageDocument(inMemoryArtifactDoa.savePage(id, markdown, revision))

    override suspend fun renameArtifact(id: String, title: String): Artifact {
        val artifact = userArtifactToArtifact(inMemoryArtifactDoa.renameArtifact(id, title))
        artifactFlow.emit(artifact)
        return artifact
    }

    override suspend fun uploadFile(
        filename: String,
        bytes: ByteArray,
        contentType: String?,
    ): UploadedFile =
        toUploadedFile(
            ApiClient.uploadFile(
                FileUploadRequest(filename = filename, bytes = bytes, contentType = contentType)
            )
        )

    override fun artifactResourceUrl(id: String): String = ApiClient.userArtifactResourceUrl(id)

    private fun toPageDocument(record: PageDocumentRecord): PageDocument =
        PageDocument(
            id = record.id,
            title = record.title,
            content = record.content,
            revision = record.revision,
            updatedAt = record.updatedAt,
        )

    private fun toUploadedFile(record: UploadedFileRecord): UploadedFile =
        UploadedFile(
            id = record.id,
            url = ApiClient.backendUrl(record.url),
            uploadedAt = record.uploadedAt,
        )

    private fun artifactBreadcrumbs(artifact: Artifact): List<BreadcrumbItem> =
        listOf(
            BreadcrumbItem(id = null, label = "Folders"),
            BreadcrumbItem(id = artifact.id, label = artifact.title),
        )
}

fun userArtifactToArtifact(record: UserArtifact): Artifact {
    val kind =
        when (record.type.lowercase()) {
            "page" -> ArtifactKind.Note
            "note" -> ArtifactKind.Note
            "link" -> ArtifactKind.Link
            "voice" -> ArtifactKind.Voice
            "file" -> ArtifactKind.File
            else -> ArtifactKind.Note
        }
    val title =
        record.title.ifBlank {
            metadataString(record.metadata, "originalFilename")
                ?: metadataString(record.metadata, "storageKey")
                ?: "Untitled artifact"
        }
    return Artifact(
        id = record.id,
        folderId = record.folderId,
        title = title,
        content = record.content,
        summary = record.summary,
        kind = kind,
        updatedLabel = record.updatedAt,
        sourceUrl = record.sourceUrl,
        resourceUrl =
            if (kind == ArtifactKind.Voice || kind == ArtifactKind.File)
                ApiClient.userArtifactResourceUrl(record.id)
            else null,
    )
}

private fun metadataString(metadata: Map<String, JsonElement>, key: String): String? {
    val value = metadata[key] ?: return null
    return (value as? JsonPrimitive)?.contentOrNull
}

private fun displayDomain(url: String?): String {
    val value = url?.trim().orEmpty()
    if (value.isEmpty()) return "example.com"
    return value.removePrefix("https://").removePrefix("http://").substringBefore("/")
}
