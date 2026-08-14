package dawn.system.anchor.services.auth

/**
 * Persists the session bearer across launches. Platform implementations are registered
 * in the platform Koin modules: `SharedPreferencesTokenStorage` on Android,
 * `UserDefaultsTokenStorage` on iOS. Moving to Keychain/EncryptedSharedPreferences is a
 * later hardening step — the interface is the seam.
 */
interface TokenStorage {
    fun read(): String?

    /** Passing null clears the stored token. */
    fun write(value: String?)
}
