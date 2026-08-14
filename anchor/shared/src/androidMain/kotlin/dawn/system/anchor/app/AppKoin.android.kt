package dawn.system.anchor.app

import android.content.Context
import dawn.system.anchor.services.auth.SharedPreferencesTokenStorage
import dawn.system.anchor.services.auth.TokenStorage
import dawn.system.anchor.services.database.DatabaseDriverFactory
import org.koin.dsl.module

/**
 * Android entry point — call from the Application with the app context. Returns [Unit]
 * (not Koin's `KoinApplication`) so callers don't need koin-core on their classpath.
 */
fun initKoin(context: Context) {
    startAnchorKoin(
        platformModule = module {
            single { DatabaseDriverFactory(context.applicationContext) }
            single<TokenStorage> { SharedPreferencesTokenStorage(context.applicationContext) }
        },
    )
}
