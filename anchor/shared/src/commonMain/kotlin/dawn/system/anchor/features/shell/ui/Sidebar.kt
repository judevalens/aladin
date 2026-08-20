package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenRow
import dawn.system.anchor.features.shell.state.openEvent
import dawn.system.anchor.features.shell.state.rowKey
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.ColumnsIcon
import dawn.system.anchor.services.design.FilterIcon
import dawn.system.anchor.services.design.SearchIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.SidebarIcon
import dawn.system.anchor.services.design.SparkleIcon
import dawn.system.anchor.services.design.destinationIcon

/**
 * The sidebar: the only navigation, and it never changes meaning.
 *
 * Six zones top to bottom, and **only the Open list scrolls**. That is deliberate rather than
 * incidental — the handoff is explicit that destinations and the Copilot entry must not be
 * reachable-only-by-scrolling, which is exactly what a single scrolling column would make
 * them once a dozen documents are open.
 */
@Composable
internal fun Sidebar(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(modifier.fillMaxHeight().background(c.panel)) {
        IconRow(state)
        FilterPills(state)
        Destinations(state)

        Box(
            Modifier.fillMaxWidth().height(1.dp).padding(horizontal = 20.dp).background(c.line2),
        )

        // `weight` is what makes this the only zone that gives: everything above and below
        // keeps its natural height, so the hero stays visible with twelve documents open.
        OpenZone(state, Modifier.weight(1f))

        CopilotHero(state)
        Spacer(Modifier.height(m.dockBottomInset - 6.dp))
    }
}

@Composable
private fun IconRow(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier.fillMaxWidth().padding(start = 10.dp, end = 10.dp, top = m.sidebarSafeInset, bottom = 6.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Search has no palette behind it yet — one of the handoff's open questions — so it
        // is drawn and inert rather than wired to something that does not exist.
        IconButton(enabled = false) { SearchIcon(tint = c.ink3, size = 19.dp) }

        // The browser's door, and **only one browser is ever live**. Standing on the browser
        // surface, the button just closes any overlay. With a tab open elsewhere it activates
        // that tab rather than stacking a dropdown over it. Otherwise it opens the dropdown.
        val onBrowser = state.nav.nav.here.surface == Surface.Browser
        val browserLit = state.chrome.browserOpen || onBrowser
        IconButton(
            background = if (browserLit) c.sel else null,
            onClick = {
                when {
                    onBrowser -> state.chrome.handle(ChromeEvent.CloseBrowser)
                    state.nav.nav.hasBrowserTab -> {
                        state.chrome.handle(ChromeEvent.CloseBrowser)
                        state.nav.handle(NavEvent.OpenBrowserTab)
                    }
                    else -> state.chrome.handle(ChromeEvent.ToggleBrowser)
                }
            },
            onLongClick = { state.chrome.handle(ChromeEvent.OpenBrowserMenu) },
        ) {
            ColumnsIcon(tint = if (browserLit) c.ink else c.ink3, size = 19.dp)
        }

        // Lit by what is ACTIVE, not by whether the popover happens to be open — the badge
        // has to say "you are looking at a narrowed list" even after the popover is dismissed.
        val narrowing = state.chrome.filter.isNarrowing
        Box {
            IconButton(
                background = if (narrowing) c.amberSoft else null,
                onClick = { state.chrome.handle(ChromeEvent.ToggleFilter) },
            ) {
                FilterIcon(tint = if (narrowing) c.amber else c.ink3, size = 19.dp)
            }
            if (narrowing) {
                Text(
                    "${state.chrome.filter.activeCount}",
                    style = SectionLabelStyle,
                    color = c.onAmber,
                    modifier = Modifier
                        .align(Alignment.TopEnd)
                        .clip(AnchorShape.pill)
                        .background(c.amber)
                        .padding(horizontal = 5.dp, vertical = 1.dp),
                )
            }
        }

        Spacer(Modifier.weight(1f))

        IconButton(onClick = { state.chrome.handle(ChromeEvent.ToggleSidebar) }) {
            SidebarIcon(tint = c.ink3, size = 19.dp)
        }
    }
}

/**
 * The active facets, as removable pills — the one place a narrowed list says so without being
 * opened. Only rendered while filtering, so the zone costs nothing the rest of the time.
 */
@Composable
private fun FilterPills(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val filter = state.chrome.filter
    if (!filter.isNarrowing) return

    FlowRow(
        Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, bottom = 8.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        filter.purposes.forEach { purpose ->
            Pill(purpose.name) { state.chrome.handle(ChromeEvent.TogglePurpose(purpose)) }
        }
        filter.kinds.forEach { kind ->
            Pill(kind.wire.uppercase()) { state.chrome.handle(ChromeEvent.ToggleKind(kind)) }
        }
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

@Composable
private fun Pill(label: String, onRemove: () -> Unit) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .height(32.dp)
            .clip(AnchorShape.chip)
            .background(c.amberSoft)
            .border(1.dp, c.amberLine, AnchorShape.chip)
            .clickable(onClick = onRemove)
            .padding(horizontal = 10.dp),
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = SectionLabelStyle, color = c.amber)
        CloseIcon(tint = c.amber, size = 12.dp)
    }
}

@Composable
private fun Destinations(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val here = state.nav.nav.here.surface

    Column(Modifier.fillMaxWidth().padding(start = 10.dp, end = 10.dp, bottom = 8.dp)) {
        state.destinations.forEach { destination ->
            val lit = here is Surface.Dest && here.destination == destination
            Row(
                Modifier
                    .fillMaxWidth()
                    .height(m.destinationRow)
                    .padding(bottom = 2.dp)
                    .clip(AnchorShape.row)
                    .background(if (lit) c.sel else c.panel.copy(alpha = 0f))
                    .clickable { state.nav.handle(NavEvent.GoTo(destination)) }
                    .padding(horizontal = 12.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                // A lit destination takes `--sel` and ink, never an amber fill: the design
                // spends its one accent per surface on the Copilot hero.
                destinationIcon(destination, tint = if (lit) c.amber else c.ink3, size = 19.dp)
                Text(
                    text = destination.title,
                    style = MaterialTheme.typography.titleMedium,
                    color = if (lit) c.ink else c.ink2,
                )
            }
        }
    }
}

@Composable
private fun OpenZone(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors

    Column(modifier.fillMaxWidth()) {
        Row(
            Modifier.fillMaxWidth().padding(start = 20.dp, end = 10.dp, top = 14.dp, bottom = 4.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text("OPEN", style = SectionLabelStyle, color = c.ink4)
            if (state.open.isNotEmpty()) {
                Text("${state.open.size}", style = SectionLabelStyle, color = c.ink4)
            }
            Spacer(Modifier.weight(1f))
            if (state.open.isNotEmpty()) {
                // The expander. A chevron rather than the sparkle that was standing in here —
                // sparkle means Copilot everywhere else in this shell.
                IconButton(
                    size = 32.dp,
                    onClick = { state.chrome.handle(ChromeEvent.ToggleSwitcher) },
                ) {
                    ChevronIcon(tint = c.ink4, size = 16.dp, direction = ChevronDirection.Down)
                }
            }
        }

        if (state.open.isEmpty()) {
            Text(
                "Nothing open.",
                style = MaterialTheme.typography.bodyMedium,
                color = c.ink4,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 22.dp),
            )
        } else {
            LazyColumn(
                Modifier.fillMaxWidth(),
                contentPadding = PaddingValues(start = 10.dp, end = 10.dp, bottom = 8.dp),
            ) {
                items(state.open, key = { it.tab.rowKey() }) { row ->
                    OpenListRow(row, state)
                }
            }
        }
    }
}

@Composable
private fun OpenListRow(row: OpenRow, state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .height(m.openRow)
            .clip(AnchorShape.menuRow)
            .background(if (row.active) c.sel else c.panel.copy(alpha = 0f))
            .clickable { state.nav.handle(row.tab.openEvent()) }
            .padding(start = 12.dp, end = 4.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // The state dot. Amber while active; otherwise the item's own state, which nothing can
        // report yet — so it is the quiet ink rather than an invented colour.
        Box(Modifier.size(7.dp).clip(CircleShape).background(if (row.active) c.amber else c.ink4))
        Text(
            text = row.title,
            style = MaterialTheme.typography.bodyMedium,
            color = if (row.active) c.ink else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        // 28pt glyph inside a 44pt row is fine; a 28pt *button* would not be.
        IconButton(size = 28.dp, onClick = { state.nav.handle(NavEvent.Close(row.tab)) }) {
            CloseIcon(tint = c.ink4, size = 14.dp)
        }
    }
}

/**
 * The one amber-filled element in the resting shell — §5.4's budget of a single accent per
 * surface, spent here.
 */
@Composable
private fun CopilotHero(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val open = state.chrome.copilotOpen

    Row(
        Modifier
            .fillMaxWidth()
            .padding(start = 10.dp, end = 10.dp, top = 8.dp, bottom = 14.dp)
            .clip(AnchorShape.heroCard)
            .background(if (open) c.amberSoft else c.card)
            .clickable { state.chrome.handle(ChromeEvent.ToggleCopilot) }
            .padding(14.dp),
        horizontalArrangement = Arrangement.spacedBy(13.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier.size(38.dp).clip(AnchorShape.field).background(c.amberSoft),
            contentAlignment = Alignment.Center,
        ) {
            SparkleIcon(tint = c.amber, size = 20.dp)
        }
        Column(Modifier.weight(1f)) {
            Text("Copilot", style = MaterialTheme.typography.titleMedium, color = c.amber)
            Text(
                text = if (open) "open" else "ask about this page",
                style = SectionLabelStyle,
                color = c.ink3,
            )
        }
    }
}

/** A square tap target. The floor is 44pt; the glyph inside it is free to be smaller. */
@Composable
internal fun IconButton(
    modifier: Modifier = Modifier,
    size: androidx.compose.ui.unit.Dp = 42.dp,
    background: androidx.compose.ui.graphics.Color? = null,
    enabled: Boolean = true,
    onClick: () -> Unit = {},
    /**
     * A long press, where one means something. `combinedClickable` also **suppresses the click
     * that would otherwise follow** — the prototype needs a 480ms timer and a `_fired` flag to
     * do that by hand in the DOM; here it is the platform's own gesture.
     */
    onLongClick: (() -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    Box(
        modifier
            .size(size)
            .clip(AnchorShape.field)
            .background(background ?: AnchorTheme.colors.panel.copy(alpha = 0f))
            .then(
                if (!enabled) {
                    Modifier
                } else {
                    Modifier.combinedClickable(onClick = onClick, onLongClick = onLongClick)
                },
            ),
        contentAlignment = Alignment.Center,
        content = { content() },
    )
}
