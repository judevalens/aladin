package com.jvp.aladin_compose.model

data class Item(
    val id: String,
    val parentId: String?,
    val title: String,
    val kind: ItemKind,
    val position: String,
    val targetId: String? = null,
)

enum class ItemKind {
    Folder,
    Artifact,
    Page,
    Group,
}

fun ItemKind.canHaveChildren(): Boolean {
    return when (this) {
        ItemKind.Folder,
        ItemKind.Page,
        ItemKind.Group -> true
        ItemKind.Artifact -> false
    }
}
