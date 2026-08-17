package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.FolderPurpose
import dawn.system.anchor.domain.ItemSort
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.CheckIcon
import dawn.system.anchor.services.design.SectionLabelStyle

/**
 * The facets, and what they leave out.
 *
 * The design asks for six kinds and three states. **Only the ones the workspace can actually
 * answer are offered**: the server stores `page | link | app | voice | file` and nothing else,
 * and it has no read tracking, no staleness and no recovered flag — so Aids, Threads, Research
 * views and all three states would be checkboxes that can only ever produce an empty list.
 *
 * That is the same call the row's state dot makes by not drawing, and `FolderPurpose` makes by
 * having no `Learning`: model the vocabulary, offer only what can be answered. A facet that
 * silently matches nothing is worse than one that is absent, because the absence is a question
 * and the empty result is a bug report.
 */
@Composable
internal fun BoxScope.FilterPopover(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val filter = state.chrome.filter

    AnchoredOverlay(
        visible = state.chrome.filterOpen,
        anchor = if (state.chrome.sidebarOpen) {
            OverlayAnchor.Below(left = 10.dp, top = m.overlayTop)
        } else {
            OverlayAnchor.AboveDock(bottom = 96.dp)
        },
        onDismiss = { state.chrome.handle(ChromeEvent.ToggleFilter) },
        modifier = Modifier.width(m.filterPopoverWidth),
    ) {
        Column(Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
            Group("KIND")
            KINDS.forEach { (kind, label) ->
                Facet(label, checked = kind in filter.kinds) {
                    state.chrome.handle(ChromeEvent.ToggleKind(kind))
                }
            }

            HorizontalHairline()
            Group("FOLDER PURPOSE")
            PURPOSES.forEach { (purpose, label) ->
                Facet(label, checked = purpose in filter.purposes) {
                    state.chrome.handle(ChromeEvent.TogglePurpose(purpose))
                }
            }

            HorizontalHairline()
            Row(
                Modifier.fillMaxWidth().padding(start = 14.dp, end = 10.dp, top = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("SORT", style = SectionLabelStyle, color = c.ink4)
                Segment("Recent", filter.sort == ItemSort.Recent) {
                    state.chrome.handle(ChromeEvent.SortBy(ItemSort.Recent))
                }
                Segment("A–Z", filter.sort == ItemSort.Name) {
                    state.chrome.handle(ChromeEvent.SortBy(ItemSort.Name))
                }
                Spacer(Modifier.weight(1f))
                if (filter.isNarrowing) {
                    Text(
                        "Clear",
                        style = MaterialTheme.typography.bodyMedium,
                        color = c.ink3,
                        modifier = Modifier
                            .clip(AnchorShape.control)
                            .clickable { state.chrome.handle(ChromeEvent.ClearFilter) }
                            .padding(horizontal = 8.dp, vertical = 6.dp),
                    )
                }
            }
            Text(
                "APPLIES TO BROWSER AND OPEN",
                style = SectionLabelStyle,
                color = c.ink4,
                modifier = Modifier.padding(start = 14.dp, top = 8.dp, bottom = 2.dp),
            )
        }
    }
}

/** The kinds the workspace can actually answer. See the note above about the ones it cannot. */
private val KINDS = listOf(
    ArtifactKind.File to "Files",
    ArtifactKind.Page to "Notes",
    ArtifactKind.Link to "Links",
    ArtifactKind.App to "Shards",
    ArtifactKind.Voice to "Voice",
)

/** `Learning` is absent for the same reason: `FolderPurpose.of` can never return it. */
private val PURPOSES = listOf(
    FolderPurpose.Research to "Research",
    FolderPurpose.Plain to "Plain",
)

@Composable
private fun Group(label: String) {
    Text(
        label,
        style = SectionLabelStyle,
        color = AnchorTheme.colors.ink4,
        modifier = Modifier.padding(start = 14.dp, top = 10.dp, bottom = 4.dp),
    )
}

@Composable
private fun Facet(label: String, checked: Boolean, onToggle: () -> Unit) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .height(40.dp)
            .padding(horizontal = 8.dp)
            .clip(AnchorShape.control)
            .background(if (checked) c.sel else c.card.copy(alpha = 0f))
            .clickable(onClick = onToggle)
            .padding(horizontal = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(19.dp)
                .clip(AnchorShape.chip)
                .background(if (checked) c.amber else c.card.copy(alpha = 0f))
                .then(if (checked) Modifier else Modifier.border(1.dp, c.line, AnchorShape.chip)),
            contentAlignment = Alignment.Center,
        ) {
            if (checked) CheckIcon(tint = c.onAmber, size = 13.dp)
        }
        Text(label, style = MaterialTheme.typography.bodyMedium, color = c.ink2)
    }
}

@Composable
private fun Segment(label: String, selected: Boolean, onClick: () -> Unit) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .height(33.dp)
            .clip(AnchorShape.chip)
            .background(if (selected) c.sel else c.card.copy(alpha = 0f))
            .clickable(onClick = onClick)
            .padding(horizontal = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            label,
            style = MaterialTheme.typography.bodyMedium,
            color = if (selected) c.ink else c.ink3,
        )
    }
}
