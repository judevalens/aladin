package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import com.jvp.aladin_compose.model.NavDestination
import com.jvp.aladin_compose.service.ItemService
import com.slack.circuit.runtime.CircuitContext
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import kotlinx.coroutines.CoroutineScope

class AppPresenter(
    private val documentBrowserProducer: DocumentBrowserProducer,
) : Presenter<AppState> {

    @Composable
    override fun present(): AppState {
        var destination by remember { mutableStateOf(NavDestination.Home) }
        val browser = documentBrowserProducer.produce()

        return AppState(
            destination = destination,
            browser = browser.browser,
            selectedItem = browser.selectedItem,
            selectedArtifact = browser.selectedArtifact,
            canCreateArtifact = browser.canCreateArtifact,
            eventSink = { event ->
                when (event) {
                    is AppEvent.NavigateDestination -> destination = event.destination
                }
            },
        )
    }

    class Factory(
        private val itemService: ItemService,
        private val scope: CoroutineScope,
    ) : Presenter.Factory {
        override fun create(screen: Screen, navigator: Navigator, context: CircuitContext): Presenter<*>? {
            return when (screen) {
                AppScreen ->
                    AppPresenter(
                        documentBrowserProducer =
                            DefaultDocumentBrowserProducer(
                                service = itemService,
                                scope = scope,
                            ),
                    )
                else -> null
            }
        }
    }
}
