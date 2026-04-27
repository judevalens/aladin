package com.jvp.aladin_compose.model

data class FolderNode(
    val id: String,
    val parentId: String?,
    val title: String,
)

data class FolderTreeNode(
    val id: String,
    val parentId: String?,
    val title: String,
    val children: List<FolderTreeNode> = emptyList(),
)

data class BreadcrumbItem(
    val id: String?,
    val label: String,
)
