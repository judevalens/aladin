package com.jvp.aladin_compose.features.app

enum class BrowserRowKind {
    Folder,
    Artifact,
}

enum class BrowserRowMenuActionTone {
    Normal,
    Secondary,
    Destructive,
}

data class BrowserRowMenuAction(
    val id: String,
    val label: String,
    val enabled: Boolean = true,
    val tone: BrowserRowMenuActionTone = BrowserRowMenuActionTone.Normal,
)

data class BrowserRowMenuSection(
    val title: String,
    val actions: List<BrowserRowMenuAction>,
)

data class BrowserRowMenuModel(
    val rowId: String,
    val rowKind: BrowserRowKind,
    val sections: List<BrowserRowMenuSection>,
)

enum class BrowserMenuPlacement {
    ContextualRight,
    DropdownBelow,
}

data class BrowserRowMenuRequest(
    val menu: BrowserRowMenuModel,
    val anchorLeftPx: Float,
    val anchorRightPx: Float,
    val anchorBottomPx: Float,
    val placement: BrowserMenuPlacement = BrowserMenuPlacement.ContextualRight,
    val matchAnchorWidth: Boolean = false,
    val elevated: Boolean = false,
    val onActionSelected: (String) -> Unit,
)
