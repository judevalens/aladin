package dawn.system.anchor.app

import dawn.system.anchor.services.auth.TokenStorage
import dawn.system.anchor.services.auth.UserDefaultsTokenStorage
import dawn.system.anchor.services.database.DatabaseDriverFactory
import org.koin.dsl.module

/**
 * iOS entry point — call from Swift as `KoinIosKt.startKoinIos()` in the app's init.
 * (Named `startKoinIos` rather than `initKoin` so Swift interop doesn't prefix it with
 * `do` — Kotlin functions named `init*` are renamed on the Objective-C/Swift side.)
 */
fun startKoinIos() {
    startAnchorKoin(
        platformModule = module {
            single { DatabaseDriverFactory() }
            single<TokenStorage> { UserDefaultsTokenStorage() }
        },
    )
}
