package dawn.system.anchor

import androidx.compose.runtime.Composable
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import org.koin.core.module.dsl.factoryOf
import org.koin.core.parameter.parametersOf
import org.koin.dsl.koinApplication
import org.koin.dsl.module
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

@CommonParcelize
private data object AssistedTestScreen : Screen {
    data class State(val tag: String) : CircuitUiState
}

private class FakeRepo(val tag: String)

private class AssistedTestPresenter(
    val screen: AssistedTestScreen,
    val navigator: Navigator,
    val repo: FakeRepo,
) : Presenter<AssistedTestScreen.State> {
    @Composable
    override fun present(): AssistedTestScreen.State = AssistedTestScreen.State(repo.tag)
}

/**
 * Verifies the assisted-injection path: `factoryOf(::Presenter)` resolves graph deps
 * automatically while `screen` and `navigator` are supplied as injected parameters.
 */
class AssistedInjectionTest {

    @Test
    fun presenterGetsGraphDepAndAssistedParams() {
        val app = koinApplication {
            modules(
                module {
                    single { FakeRepo("injected") }
                    factoryOf(::AssistedTestPresenter)
                    bindScreen<AssistedTestScreen, AssistedTestPresenter>(
                        uiFactory = { ui<AssistedTestScreen.State> { _, _ -> } },
                    )
                },
            )
        }
        val koin = app.koin

        val presenter = koin.get<AssistedTestPresenter> {
            parametersOf(AssistedTestScreen, Navigator.NoOp)
        }

        assertEquals("injected", presenter.repo.tag, "graph dependency should be auto-wired")
        assertEquals(AssistedTestScreen, presenter.screen, "screen should be the assisted param")
        assertTrue(koin.getAll<Presenter.Factory>().isNotEmpty(), "factory should be registered")

        app.close()
    }
}
