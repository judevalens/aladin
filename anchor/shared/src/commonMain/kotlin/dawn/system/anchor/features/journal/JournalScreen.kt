package dawn.system.anchor.features.journal

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.services.design.PlaceholderScreen
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import org.koin.core.module.Module
import org.koin.dsl.module

/** Journal timeline — chronological mixed-media entries with search. (stub) */
@CommonParcelize
data object JournalScreen : Screen {
    data class State(val eventSink: (Event) -> Unit) : CircuitUiState
    sealed interface Event
}

class JournalPresenter : Presenter<JournalScreen.State> {
    @Composable
    override fun present(): JournalScreen.State = JournalScreen.State {}
}

@Composable
fun JournalUi(state: JournalScreen.State, modifier: Modifier = Modifier) {
    PlaceholderScreen(
        title = "Journal",
        subtitle = "everything you've let out, held gently in order.",
        modifier = modifier,
    )
}

val journalModule: Module = module {
    bindScreen(
        JournalScreen::class,
        presenter = { _, _ -> JournalPresenter() },
        uiFactory = { ui<JournalScreen.State> { state, modifier -> JournalUi(state, modifier) } },
    )
}
