package dawn.system.anchor.services.auth

import platform.Foundation.NSUserDefaults

/** NSUserDefaults for now; Keychain is the hardening step. */
class UserDefaultsTokenStorage(
    private val defaults: NSUserDefaults = NSUserDefaults.standardUserDefaults,
) : TokenStorage {

    override fun read(): String? = defaults.stringForKey(KEY)

    override fun write(value: String?) {
        if (value == null) {
            defaults.removeObjectForKey(KEY)
        } else {
            defaults.setObject(value, forKey = KEY)
        }
        // NSUserDefaults flushes on its own schedule, so a force-quit shortly after
        // sign-in would otherwise drop the token and bounce the user back to login.
        // Verified: without this, killing the app right after login lost the session.
        defaults.synchronize()
    }

    private companion object {
        const val KEY = "aladin_session.bearer_token"
    }
}
