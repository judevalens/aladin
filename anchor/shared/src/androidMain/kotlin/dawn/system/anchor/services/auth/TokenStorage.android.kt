package dawn.system.anchor.services.auth

import android.content.Context

/** Plain SharedPreferences for now; EncryptedSharedPreferences is the hardening step. */
class SharedPreferencesTokenStorage(context: Context) : TokenStorage {
    private val prefs = context.applicationContext
        .getSharedPreferences("aladin_session", Context.MODE_PRIVATE)

    override fun read(): String? = prefs.getString(KEY, null)

    override fun write(value: String?) {
        val editor = prefs.edit()
        if (value == null) editor.remove(KEY) else editor.putString(KEY, value)
        // commit() rather than apply(): the write is rare and small, and the token
        // must be on disk before the process can die (see the iOS actual's note).
        editor.commit()
    }

    private companion object {
        const val KEY = "bearer_token"
    }
}
