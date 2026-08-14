package dawn.system.anchor.features.capture

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.slack.circuit.runtime.CircuitUiState
import com.slack.circuit.runtime.Navigator
import com.slack.circuit.runtime.presenter.Presenter
import com.slack.circuit.runtime.screen.Screen
import com.slack.circuit.runtime.ui.ui
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.di.CommonParcelize
import dawn.system.anchor.services.di.bindScreen
import org.koin.core.module.Module
import org.koin.dsl.module

/** Quick Capture — the hero create surface, pushed over the active tab. (stub) */
@CommonParcelize
data object CaptureScreen : Screen {
    data class State(val eventSink: (Event) -> Unit) : CircuitUiState
    sealed interface Event {
        data object Close : Event
    }
}

class CapturePresenter(private val navigator: Navigator) : Presenter<CaptureScreen.State> {
    @Composable
    override fun present(): CaptureScreen.State = CaptureScreen.State { event ->
        when (event) {
            CaptureScreen.Event.Close -> navigator.pop()
        }
    }
}

@Composable
fun CaptureUi(state: CaptureScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val space = AnchorTheme.space
    Column(
        modifier
            .fillMaxSize()
            .background(c.bg)
            .clickable { state.eventSink(CaptureScreen.Event.Close) }
            .padding(horizontal = space.gutter)
            .padding(top = space.xxl, bottom = space.xxl),
    ) {
        Text("Capture", style = MaterialTheme.typography.displaySmall, color = c.ink)
        Spacer(Modifier.height(8.dp))
        Text("Text · voice · photo · video — coming soon. Tap to close.", style = MaterialTheme.typography.bodyLarge, color = c.ink3)
    }
}

val captureModule: Module = module {
    bindScreen(
        CaptureScreen::class,
        presenter = { _, navigator -> CapturePresenter(navigator) },
        uiFactory = { ui<CaptureScreen.State> { state, modifier -> CaptureUi(state, modifier) } },
    )
}
