package com.jvp.aladin_compose.model

enum class ArtifactKind {
    Note,
    Link,
    Voice,
}

data class Artifact(
    val id: String,
    val folderId: String,
    val title: String,
    val summary: String,
    val kind: ArtifactKind,
    val updatedLabel: String,
)
