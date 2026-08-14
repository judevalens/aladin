package dawn.system.anchor.services.data

/** A domain model with a stable identity — the key used for sync and local persistence. */
interface Identifiable {
    val id: String
}
