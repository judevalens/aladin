package dawn.system.anchor.services.design

import androidx.compose.foundation.background
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

/**
 * Temporary placeholder for screens that are wired into navigation but not yet built —
 * a titled surface using the Anchor tokens. Replaced by each feature's real Ui.
 */
@Composable
fun PlaceholderScreen(title: String, subtitle: String, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val space = AnchorTheme.space
    Column(
        modifier
            .fillMaxSize()
            .background(c.bg)
            .padding(horizontal = space.gutter)
            .padding(top = space.xxl, bottom = space.xxl),
    ) {
        Text(title, style = MaterialTheme.typography.displaySmall, color = c.ink)
        Spacer(Modifier.height(8.dp))
        Text(subtitle, style = MaterialTheme.typography.bodyLarge, color = c.ink3)
    }
}
