package dawn.system.anchor.app

import org.koin.core.KoinApplication
import org.koin.core.context.startKoin
import org.koin.core.module.Module
import org.koin.dsl.KoinAppDeclaration

/**
 * Starts Koin with the shared [appModules] plus a [platformModule] that supplies
 * platform-specific bindings (e.g. the SQLite driver factory). Called from the platform
 * entry points — Android's Application and iOS's app init.
 */
fun startAnchorKoin(
    platformModule: Module,
    appDeclaration: KoinAppDeclaration = {},
): KoinApplication = startKoin {
    appDeclaration()
    modules(appModules() + platformModule)
}
