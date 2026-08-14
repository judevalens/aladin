package dawn.system.anchor

import com.slack.circuit.foundation.Circuit
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.ui.Ui
import dawn.system.anchor.app.circuitModule
import dawn.system.anchor.features.login.loginModule
import org.koin.dsl.koinApplication
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull

/**
 * Verifies the no-codegen screen-binding mechanism: that qualified Presenter/Ui factory
 * singletons registered by [bindScreen][dawn.system.anchor.services.di.bindScreen] are actually
 * collected by Koin's `getAll`, and that the Circuit assembles from them. Isolated Koin
 * (no DB / platform module needed).
 */
class CircuitWiringTest {

    @Test
    fun screenFactoriesAreCollectedAndCircuitBuilds() {
        val app = koinApplication { modules(circuitModule, loginModule) }
        val koin = app.koin

        assertEquals(
            1,
            koin.getAll<Presenter.Factory>().size,
            "LoginScreen's Presenter.Factory should be collected via getAll",
        )
        assertEquals(
            1,
            koin.getAll<Ui.Factory>().size,
            "LoginScreen's Ui.Factory should be collected via getAll",
        )

        val circuit: Circuit = koin.get()
        assertNotNull(circuit, "Circuit should build from the collected factories")

        app.close()
    }
}
