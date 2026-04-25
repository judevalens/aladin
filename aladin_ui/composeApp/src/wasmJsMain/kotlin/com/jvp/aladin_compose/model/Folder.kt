package com.jvp.aladin_compose.model

data class FolderNode(
    val id: String,
    val parentId: String?,
    val title: String,
)

data class BreadcrumbItem(
    val id: String?,
    val label: String,
)

data class FolderWorkspace(
    val folder: FolderNode,
    val artifacts: List<Artifact>,
    val signalCount: Int,
)
