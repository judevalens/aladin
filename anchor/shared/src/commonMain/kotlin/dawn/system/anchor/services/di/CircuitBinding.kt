package dawn.system.anchor.services.di

import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.Ui
import org.koin.core.module.Module
import org.koin.core.parameter.parametersOf
import org.koin.core.qualifier.named
import org.koin.core.scope.Scope
import kotlin.reflect.KClass

/**
 * Binds a [Screen]'s [Presenter] and [Ui] together and registers them so the Circuit
 * picks them up automatically — no central `when` to edit when adding a screen.
 *
 * Each screen contributes a [Presenter.Factory] and a [Ui.Factory] as qualified Koin
 * singletons; [buildCircuit][dawn.system.anchor.buildCircuit] collects them all via
 * `getAll`. [presenter] runs with the Koin [Scope] as receiver, so it can pull injected
 * dependencies with `get()`. [uiFactory] builds the (stateless) Circuit [Ui] — typically
 * `ui<MyState> { state, modifier -> MyUi(state, modifier) }`.
 */
fun <S : Screen> Module.bindScreen(
    screenClass: KClass<S>,
    presenter: Scope.(screen: S, navigator: Navigator) -> Presenter<*>,
    uiFactory: () -> Ui<*>,
) {
    single<Presenter.Factory>(named("${screenClass.qualifiedName}#presenter")) {
        val scope = this
        Presenter.Factory { screen, navigator, _ ->
            @Suppress("UNCHECKED_CAST")
            if (screenClass.isInstance(screen)) scope.presenter(screen as S, navigator) else null
        }
    }
    single<Ui.Factory>(named("${screenClass.qualifiedName}#ui")) {
        val ui = uiFactory()
        Ui.Factory { screen, _ ->
            if (screenClass.isInstance(screen)) ui else null
        }
    }
}

/** Reified convenience: `bindScreen<TodayScreen>(presenter = …, uiFactory = …)`. */
inline fun <reified S : Screen> Module.bindScreen(
    noinline presenter: Scope.(S, Navigator) -> Presenter<*>,
    noinline uiFactory: () -> Ui<*>,
): Unit = bindScreen(S::class, presenter, uiFactory)

/**
 * Assisted-injection overload: bind a screen whose presenter [P] is registered with Koin's
 * constructor DSL (`factoryOf(::MyPresenter)`). The presenter is resolved from the graph
 * with the `screen` and `navigator` passed as injected parameters — so its other
 * constructor dependencies are wired automatically and you don't list them here.
 *
 * Usage:
 * ```
 * val todayModule = module {
 *     factoryOf(::TodayPresenter)                 // entries: EntryRepository ← graph
 *     bindScreen<TodayScreen, TodayPresenter>(     // screen + navigator ← parametersOf
 *         uiFactory = { ui<TodayScreen.State> { state, modifier -> TodayUi(state, modifier) } },
 *     )
 * }
 * ```
 * The presenter's constructor must accept the `Screen` and `Navigator` (matched by type)
 * plus any graph dependencies.
 */
inline fun <reified S : Screen, reified P : Presenter<*>> Module.bindScreen(
    noinline uiFactory: () -> Ui<*>,
) {
    val presenterClass = P::class
    bindScreen(
        S::class,
        presenter = { screen, navigator -> get(presenterClass) { parametersOf(screen, navigator) } },
        uiFactory = uiFactory,
    )
}
