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
import androidx.compose.runtime.key
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.zIndex
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.Fetch
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenDocument
import dawn.system.anchor.features.shell.state.OpenPdf
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
    val surface = state.nav.nav.here.surface
    val readingPdf = state.documents.active is OpenDocument.Pdf

    Box(modifier.background(c.bg)) {
        // EVERY open PDF, composed unconditionally and never wrapped in a visibility test.
        // Compose keeps a viewer alive because it is still in the tree — that is the whole
        // mechanism, and `if (active) …` would quietly turn it back into a teardown.
        //
        // DERIVED, never stored: the index and the list have to change in the same
        // composition or a close slides the reader by a column. See [DocumentPager].
        val activeIndex = state.documents.pdfs.indexOfFirst { it.active }.coerceAtLeast(0)

        if (DOCUMENT_RESIDENCY == Residency.Pager) {
            DocumentPager(
                pages = state.documents.pdfs,
                activeIndex = activeIndex,
                key = { it.key.asString() },
                modifier = Modifier.fillMaxSize(),
            ) { pdf ->
                PdfPage(pdf)
            }
        } else {
            state.documents.pdfs.forEach { pdf ->
                key(pdf.key.asString()) {
                    Box(Modifier.fillMaxSize().zIndex(if (readingPdf && pdf.active) 1f else 0f)) {
                        PdfPage(pdf)
                    }
                }
            }
        }

        // Everything else draws over them, on its own opaque ground.
        if (!readingPdf) {
            Box(Modifier.fillMaxSize().background(c.bg).zIndex(2f)) {
                when (surface) {
                    is Surface.Dest -> when (surface.destination) {
                        Destination.Home -> Placeholder("Home", "Continue, insights and today's activity.")
                        Destination.Browser -> BrowserSurface(state.browser, state.nav.handle)
                        Destination.Markets -> Placeholder("Markets", "Index strip, watchlist and positions.")
                        Destination.Graph -> Placeholder("Graph", "The entity canvas.")
                    }

                    is Surface.Doc -> NonPdfDocument(state.documents.active)
                }
            }
        }
    }
}

/**
 * How open documents stay resident. Two implementations, kept side by side so they can be
 * compared on a device — the difference is sub-100ms and not judgeable from a description.
 */
internal enum class Residency {
    /** A row of viewport-wide columns; the non-current ones sit off-screen. */
    Pager,

    /** All viewers at the same coordinates; `zIndex` picks the front one. Known-good. */
    Stack,
}

internal val DOCUMENT_RESIDENCY = Residency.Pager

/** One page of the pager: a PDF once its bytes have landed, and what to say until they do. */
@Composable
private fun PdfPage(pdf: OpenPdf) {
    when (val bytes = pdf.bytes) {
        // A genuine first open. Anything fetched before paints on frame one, because the
        // resource store answers from its cache synchronously.
        Fetch.Pending -> Placeholder(pdf.title, "Fetching…")
        // Said out loud rather than left spinning: a failure that looks like a slow success
        // is a surface that waits forever.
        is Fetch.Failed -> Placeholder(pdf.title, "Couldn't open it — ${bytes.reason}.")
        is Fetch.Ready -> PdfViewer(
            filePath = bytes.path,
            page = 0,
            onPageChanged = {},
            modifier = Modifier.fillMaxSize(),
        )
    }
}

/** An open document that is not a PDF — or one whose bytes have not landed. */
@Composable
private fun NonPdfDocument(document: OpenDocument?) {
    when (document) {
        null -> Placeholder("Nothing open", "Pick something in the Browser.")

        is OpenDocument.Pdf -> when (val bytes = document.bytes) {
            // A genuine first open. Anything fetched before paints on frame one, because the
            // resource store answers from its cache synchronously.
            Fetch.Pending -> Placeholder(document.title, "Fetching…")
            // Said out loud rather than left spinning: a failure that looks like a slow
            // success is a surface that waits forever.
            is Fetch.Failed -> Placeholder(document.title, "Couldn't open it — ${bytes.reason}.")
            is Fetch.Ready -> Unit // drawn by its own viewer, underneath
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
