package com.jvp.aladin_compose.model

data class FolderNode(
    val id: String,
    val parentId: String?,
    val title: String,
)

enum class BrowserNodeKind {
    Folder,
    Artifact,
}

data class ArtifactPreview(
    val id: String,
    val title: String,
    val kind: ArtifactKind,
    val updatedLabel: String? = null,
)

data class BrowserTreeNode(
    val id: String,
    val parentId: String?,
    val kind: BrowserNodeKind,
    val title: String,
    val artifactId: String? = null,
    val artifactPreview: ArtifactPreview? = null,
    val children: List<BrowserTreeNode> = emptyList(),
)

data class BreadcrumbItem(
    val id: String?,
    val label: String,
)
