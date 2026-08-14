package dawn.system.anchor.domain

/**
 * What a folder is *for*. The handoff's central rule: **purpose drives which sections a
 * folder's detail renders, not which layout or which component.** One renderer, fewer
 * sections — a learning folder shows objective → topics → contents, a research folder
 * shows hypothesis → slots, a plain folder shows contents alone.
 *
 * **Reality check.** The backend models `kind` (folder | research | artifact); it has no
 * notion of a *learning* folder yet. So [Research] and [Plain] resolve from real data
 * today and [Learning] cannot — it is modelled here because the rule is the design's
 * spine, and the sections that depend on it stay empty until the backend can say so
 * (see the tutor plan object). Nothing fabricates a purpose the server does not know.
 */
enum class FolderPurpose {
    Research,
    Learning,
    Plain,
    ;

    companion object {
        /**
         * Derives purpose from what the tree actually knows. Deliberately conservative:
         * an artifact has no purpose, a research node is [Research], everything else is
         * [Plain] until the server can distinguish a learning folder.
         */
        fun of(node: WorkspaceNode): FolderPurpose? = when (node.kind) {
            NodeKind.Research -> Research
            NodeKind.Folder -> Plain
            NodeKind.Artifact -> null
        }
    }
}
