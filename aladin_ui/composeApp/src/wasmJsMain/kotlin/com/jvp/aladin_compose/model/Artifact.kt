package com.jvp.aladin_compose.model

enum class ArtifactKind {
    Note,
    Link,
    Voice,
    File,
}

data class Artifact(
    val id: String,
    val folderId: String?,
    val title: String,
    val content: String,
    val summary: String?,
    val kind: ArtifactKind,
    val updatedLabel: String,
    val sourceUrl: String? = null,
    val resourceUrl: String? = null,
)
