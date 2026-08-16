package dawn.system.anchor.domain

/**
 * What an artifact *is*.
 *
 * The closed set is the **server's**, not the design's: `artifacts.type` is validated in Go
 * as `page | link | app | voice | file` and nothing else reaches a client. There is no SQL
 * constraint behind it, and the three write paths disagree about which subset each allows —
 * so this enum is the one place that states the whole set.
 *
 * Two mappings are worth knowing, because the vocabularies differ:
 *
 *  - **`page` is what the design calls a "note".** The desktop renames it at its API boundary
 *    (`artifact-mappers.ts`); we keep the server's name and let the label do the translating,
 *    so there is one identity and one place that spells it for a human.
 *  - **A PDF is a [File]**, distinguished only by its mime type. There is no `pdf` kind.
 *
 * And three the design asks for that do not exist server-side at all — `aid`, `thread` and a
 * research `view`. They are deliberately absent rather than stubbed: nothing should be able to
 * construct a kind the workspace cannot store.
 */
enum class ArtifactKind(val wire: String) {
    Page("page"),
    Link("link"),
    App("app"),
    Voice("voice"),
    File("file"),
    ;

    /**
     * Whether this shares the one web view.
     *
     * `page` and `app` are rendered by the desktop editor embedded in a `WKWebView`, which
     * holds every open document at once — so they cost one surface between them, not one
     * each. Everything else gets a surface of its own.
     */
    val isWebBacked: Boolean get() = this == Page || this == App

    companion object {
        fun fromWire(value: String?): ArtifactKind? = entries.firstOrNull { it.wire == value }
    }
}

/** The kind, or null for a folder or research node — neither of which is an artifact. */
val WorkspaceNode.artifactKind: ArtifactKind?
    get() = if (kind == NodeKind.Artifact) ArtifactKind.fromWire(artifactType) else null
