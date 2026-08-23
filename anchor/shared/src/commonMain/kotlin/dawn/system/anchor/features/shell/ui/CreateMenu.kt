package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.WriteEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.artifactKindIcon

/**
 * What the plus makes.
 *
 * **A menu, not a button that creates the default thing.** A folder, a note and a board are
 * three different intentions and the workspace ranks none of them above the others; a plus that
 * made a note would put the other two behind a gesture nobody finds.
 *
 * The glyphs come from [artifactKindIcon] — the same map the browser rows draw from — so a new
 * board is announced by the icon it will have. A separate set of "new thing" glyphs would drift
 * from the rows the moment a kind changed.
 *
 * **It says where the thing will land.** New items go into the tree's focused folder — else
 * the open document's folder, else the Browser's — which is invisible from a sidebar button:
 * the footer names it, so a board made from Home and a board made three folders deep are
 * visibly different acts rather than the same tap with a surprise attached.
 */
@Composable
internal fun BoxScope.CreateMenu(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    AnchoredOverlay(
        visible = state.chrome.createOpen,
        // Under the plus itself (fourth button: 10 + 3 × 46). With the sidebar away there is no
        // icon row to hang under, so it re-anchors over the dock like every other overlay.
        anchor = if (state.chrome.sidebarOpen) {
            OverlayAnchor.Below(left = 148.dp, top = m.overlayTop + 4.dp)
        } else {
            OverlayAnchor.AboveDock(bottom = 96.dp)
        },
        onDismiss = { state.chrome.handle(ChromeEvent.CloseCreate) },
        shape = AnchorShape.card,
        modifier = Modifier.width(m.contextMenuWidth),
    ) {
        Column(Modifier.fillMaxWidth().padding(6.dp)) {
            // The producer closes this menu itself, through onCreateChosen — so the rows dispatch
            // one event each and nothing here has to remember to tidy up after a write.
            CreateRow(
                label = "New folder",
                kind = null,
                isContainer = true,
                onClick = { state.write.handle(WriteEvent.CreateFolder) },
            )
            CreateRow(
                label = "New note",
                kind = "page",
                onClick = { state.write.handle(WriteEvent.CreatePage) },
            )
            CreateRow(
                label = "New board",
                kind = "board",
                onClick = { state.write.handle(WriteEvent.CreateBoard) },
            )

            HorizontalHairline()
            Text(
                "IN ${state.tree.createTarget.title.uppercase()}",
                style = SectionLabelStyle,
                color = c.ink4,
                modifier = Modifier.padding(start = 11.dp, top = 10.dp, bottom = 6.dp),
            )
        }
    }
}

@Composable
private fun CreateRow(
    label: String,
    kind: String?,
    isContainer: Boolean = false,
    onClick: () -> Unit,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.touchTarget)
            .clip(AnchorShape.control)
            .clickable(onClick = onClick)
            .padding(horizontal = 11.dp),
        horizontalArrangement = Arrangement.spacedBy(11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // A folder is amber because of what it is — the tint is the caller's, per AnchorIcons.
        artifactKindIcon(
            artifactType = kind,
            isContainer = isContainer,
            tint = if (isContainer) c.amber else c.ink3,
            size = 18.dp,
        )
        Text(label, style = MaterialTheme.typography.bodyMedium, color = c.ink)
        Spacer(Modifier.weight(1f))
    }
}
