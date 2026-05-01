package com.jvp.aladin_compose.features.app.browser

import com.jvp.aladin_compose.features.app.BrowserRowMenuModel
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.FolderNode

enum class BrowserCreateOption {
    Folder,
    Note,
    Link,
    Voice,
    Upload,
}

sealed interface BrowserTreeRow {
    val key: String
    val depth: Int
    val selected: Boolean
    val menu: BrowserRowMenuModel

    data class Folder(
        val folder: FolderNode,
        override val depth: Int,
        val expanded: Boolean,
        val expandable: Boolean,
        override val selected: Boolean,
        override val menu: BrowserRowMenuModel,
    ) : BrowserTreeRow {
        override val key: String = "folder_${folder.id}"
    }

    data class Artifact(
        val artifact: ArtifactPreview,
        override val depth: Int,
        override val selected: Boolean,
        override val menu: BrowserRowMenuModel,
    ) : BrowserTreeRow {
        override val key: String = "artifact_${artifact.id}"
    }
}
