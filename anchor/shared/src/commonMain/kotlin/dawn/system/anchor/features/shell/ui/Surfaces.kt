package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.Fetch
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenDocument
import dawn.system.anchor.services.platform.PdfViewer
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.SearchIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.SidebarIcon
import dawn.system.anchor.services.design.SparkleIcon
import dawn.system.anchor.services.design.destinationIcon

/**
 * The content slot.
 *
 * **Each surface owns its own shape.** That is the handoff's central correction: Markets is a
 * dashboard, Graph is a canvas, Home is a feed — none of them is a tree, and the first pass
 * failed by rendering all of them through one tree-shaped pane. Only [Destination.Browser] is
 * a tree, and that is where tree-ness is quarantined.
 *
 * The native hosts — the one `WKWebView`, and the pool of `PDFView`s — will be created *here*,
 * above the swap, rather than inside any surface. A surface that owns a live native view
 * destroys it every time you switch away, which is the reload this design exists to avoid.
 */
@Composable
internal fun SurfaceHost(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val here = state.nav.nav.here.surface

    // Everything the shell can show, as one flat list of pages: the four destinations, then
    // every open document. There is no longer a document layer and a destination layer with a
    // condition deciding which draws — there are just pages, and one of them is current.
    val pages = remember(state.destinations, state.documents.documents) {
        state.destinations.map(ShellPage::Dest) + state.documents.documents.map(ShellPage::Doc)
    }

    // DERIVED, never stored. This is the pager's one contract: the current page sits at the
    // origin by definition, and a stored index is the only way a reader ever sees movement.
    val activeIndex = pages.indexOfFirst { it.matches(here) }.coerceAtLeast(0)

    ResidentPager(
        pages = pages,
        activeIndex = activeIndex,
        key = ShellPage::key,
        modifier = modifier.background(c.bg),
    ) { page ->
        when (page) {
            is ShellPage.Dest -> when (page.destination) {
                Destination.Home -> Placeholder("Home", "Continue, insights and today's activity.")
                Destination.Browser -> BrowserSurface(state.browser, state.nav.handle)
                Destination.Markets -> Placeholder("Markets", "Index strip, watchlist and positions.")
                Destination.Graph -> Placeholder("Graph", "The entity canvas.")
            }

            is ShellPage.Doc -> DocumentPage(page.document)
        }
    }
}

/**
 * One page of the shell.
 *
 * Destinations are pages too, which is what makes the Browser keep its column scroll when you
 * open a document and come back — previously that whole surface was torn down and rebuilt.
 * They cost nothing extra to keep resident: their producers already run every frame in the
 * presenter, so staying composed adds layout, not queries.
 */
private sealed interface ShellPage {
    val key: String

    data class Dest(val destination: Destination) : ShellPage {
        override val key: String get() = "dest:${destination.id}"
    }

    data class Doc(val document: OpenDocument) : ShellPage {
        override val key: String get() = document.key.asString()
    }

    fun matches(surface: Surface): Boolean = when {
        this is Dest && surface is Surface.Dest -> destination == surface.destination
        this is Doc && surface is Surface.Doc -> document.key == surface.key
        else -> false
    }
}

/** One open document. The kind decides the surface; nothing here decides whether it exists. */
@Composable
private fun DocumentPage(document: OpenDocument) {
    when (document) {
        is OpenDocument.Pdf -> when (val bytes = document.bytes) {
            // A genuine first open. Anything fetched before paints on frame one, because the
            // resource store answers from its cache synchronously.
            Fetch.Pending -> Placeholder(document.title, "Fetching…")
            // Said out loud rather than left spinning: a failure that looks like a slow
            // success is a surface that waits forever.
            is Fetch.Failed -> Placeholder(document.title, "Couldn't open it — ${bytes.reason}.")
            is Fetch.Ready -> PdfViewer(
                filePath = bytes.path,
                page = 0,
                onPageChanged = {},
                modifier = Modifier.fillMaxSize(),
            )
        }

        is OpenDocument.Unsupported -> Placeholder(
            title = document.title,
            body = "The ${document.kind?.wire ?: "artifact"} surface mounts here.",
        )
    }
}

/**
 * A surface that has not been built yet, saying so.
 *
 * Deliberately not a spinner or a zeroed dashboard: the design system's rule is **don't
 * invent metrics**, and a placeholder that mimics a real surface is the same lie in a
 * different font.
 */
@Composable
private fun Placeholder(title: String, body: String) {
    val c = AnchorTheme.colors
    Column(
        Modifier.fillMaxSize().padding(horizontal = 34.dp, vertical = 26.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, style = MaterialTheme.typography.headlineMedium, color = c.ink)
        Text(
            body,
            style = MaterialTheme.typography.bodyMedium,
            color = c.ink3,
            textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 8.dp),
        )
    }
}

/**
 * Copilot as a right sidebar — never a sheet, and never overlapping. It slides the content
 * pane narrower, which is why the frame animates a width rather than drawing over anything.
 */
@Composable
internal fun CopilotSidebar(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(modifier.fillMaxHeight().background(c.panel)) {
        Row(
            Modifier.fillMaxWidth().padding(start = 14.dp, end = 14.dp, top = m.toolbarSafeInset),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Box(
                Modifier.size(34.dp).clip(AnchorShape.field).background(c.amberSoft),
                contentAlignment = Alignment.Center,
            ) { SparkleIcon(tint = c.amber, size = 18.dp) }
            Text("Copilot", style = MaterialTheme.typography.titleMedium, color = c.ink)
            Box(Modifier.weight(1f))
            IconButton(size = m.touchTarget, onClick = { state.chrome.handle(ChromeEvent.ToggleCopilot) }) {
                CloseIcon(tint = c.ink3, size = 18.dp)
            }
        }
        Box(Modifier.fillMaxSize().padding(16.dp)) {
            Text(
                "The thread, its citation receipts and the composer land here.",
                style = MaterialTheme.typography.bodyMedium,
                color = c.ink3,
            )
        }
    }
}

/**
 * The dock shown while the sidebar is away — the destinations, the two most recent documents,
 * search and Copilot, floating over the content.
 *
 * Its open pills read [ShellScreen.State.open] in **recency** order, unlike the sidebar's
 * list: the dock has room for two, so "the two you were just in" is the only useful pair.
 */
@Composable
internal fun FloatingDock(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics
    val here = state.nav.nav.here.surface

    Row(
        modifier
            .padding(bottom = m.dockBottomInset)
            .clip(AnchorShape.sheet)
            .background(c.chrome)
            .padding(8.dp),
        horizontalArrangement = Arrangement.spacedBy(5.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconButton(size = m.dockCell, onClick = { state.chrome.handle(ChromeEvent.ToggleSidebar) }) {
            SidebarIcon(tint = c.ink3, size = 19.dp)
        }

        state.destinations.forEach { destination ->
            val lit = here is Surface.Dest && here.destination == destination
            IconButton(
                size = m.dockCell,
                background = if (lit) c.amberSoft else null,
                onClick = { state.nav.handle(NavEvent.GoTo(destination)) },
            ) {
                destinationIcon(destination, tint = if (lit) c.amber else c.ink3, size = 19.dp)
            }
        }

        state.nav.nav.switcherOrder().take(2).forEach { key ->
            val row = state.open.firstOrNull { it.key == key } ?: return@forEach
            Row(
                Modifier
                    .clip(AnchorShape.row)
                    .background(if (row.active) c.sel else c.chrome)
                    .padding(horizontal = 12.dp, vertical = 14.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    Modifier.size(7.dp).clip(CircleShape)
                        .background(if (row.active) c.amber else c.ink4),
                )
                Text(
                    row.title,
                    style = SectionLabelStyle,
                    color = if (row.active) c.ink else c.ink2,
                    maxLines = 1,
                )
            }
        }

        IconButton(size = m.dockCell, enabled = false) { SearchIcon(tint = c.ink3, size = 19.dp) }
        IconButton(
            size = m.dockCell,
            background = c.amber,
            onClick = { state.chrome.handle(ChromeEvent.ToggleCopilot) },
        ) {
            SparkleIcon(tint = c.onAmber, size = 19.dp)
        }
    }
}
