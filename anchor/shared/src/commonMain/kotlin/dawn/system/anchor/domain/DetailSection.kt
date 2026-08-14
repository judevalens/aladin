package dawn.system.anchor.domain

/**
 * The small vocabulary every destination's detail is composed from, so **one renderer
 * serves all of them** (handoff §Destinations). A destination contributes *sections*, not
 * a bespoke screen; adding a destination must never mean adding a layout.
 */
sealed interface DetailSection {
    /** Up to three cards: mono uppercase label + a large value. */
    data class Stats(val cards: List<StatCard>) : DetailSection

    /** A labelled card carrying one statement — a hypothesis, an objective, a claim. */
    data class Prose(val label: String, val statement: String) : DetailSection

    /** Learning folders only: what we're working through, with progress per topic. */
    data class Topics(val label: String, val hint: String?, val items: List<TopicItem>) : DetailSection

    /** Rows with a typed glyph, title, meta, chevron. */
    data class Contents(val label: String, val items: List<ContentRow>) : DetailSection

    /** One explanatory line under a list. */
    data class Note(val text: String) : DetailSection
}

data class StatCard(
    val label: String,
    val value: String,
    val tone: Tone = Tone.Ink,
)

data class TopicItem(
    val id: String,
    val title: String,
    val objective: String?,
    val sourceRef: String?,
    val state: TopicState,
)

/** How well the topic is understood — the tutor's judgement, never guessed by the client. */
enum class TopicState { Untouched, Reading, Shaky, Solid }

data class ContentRow(
    val id: String,
    val title: String,
    val meta: String?,
    val kind: NodeKind,
    val artifactType: String?,
)

/** Semantic colour roles, resolved to tokens by the UI layer (never a hex here). */
enum class Tone { Ink, Amber, For, Against }
