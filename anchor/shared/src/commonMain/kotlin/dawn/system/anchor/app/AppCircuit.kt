package dawn.system.anchor.app

import com.slack.circuit.foundation.Circuit
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.ui.Ui
import org.koin.core.scope.Scope

/**
 * Builds the app's [Circuit] by collecting every screen's [Presenter.Factory] and
 * [Ui.Factory] from Koin (registered via
 * [bindScreen][dawn.system.anchor.services.di.bindScreen]). Adding a screen requires no change
 * here — its feature module contributes the factories and they're picked up by `getAll`.
 */
fun buildCircuit(scope: Scope): Circuit =
    Circuit.Builder()
        .addPresenterFactories(scope.getAll<Presenter.Factory>())
        .addUiFactories(scope.getAll<Ui.Factory>())
        .build()
