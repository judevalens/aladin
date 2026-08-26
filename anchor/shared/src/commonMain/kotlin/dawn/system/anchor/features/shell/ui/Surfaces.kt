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
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.ArtifactKind
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.Surface
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.ChromeEvent
import dawn.system.anchor.features.shell.state.Fetch
import dawn.system.anchor.domain.TabKey
import dawn.system.anchor.features.shell.state.NavEvent
import dawn.system.anchor.features.shell.state.OpenDocument
import dawn.system.anchor.features.reader.DocumentReader
import dawn.system.anchor.features.reader.ReaderSeeks
import dawn.system.anchor.features.note.WebPane
import dawn.system.anchor.features.note.WebSurfacePage
import dawn.system.anchor.features.note.rememberWebSurfaceHost
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.CloseIcon
import dawn.system.anchor.services.design.SparkleIcon

/**
 * The content slot.
 *
 * **Each surface owns its own shape.** That is the handoff's central correction: Markets is a
 * dashboard, Graph is a canvas, Home is a feed — none of them is a tree, and the first pass
 * failed by rendering all of them through one tree-shaped pane. Only the browser is a tree,
 * and that is where tree-ness is quarantined — as a surface, no longer as a destination.
 *
 * The one `WKWebView` is created *here*, above the pager, rather than inside the page that
 * shows it. A surface that owns a live native view destroys it every time you switch away,
 * which is the reload this design exists to avoid.
 */
@Composable
internal fun SurfaceHost(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val here = state.nav.nav.here.surface

    // The web-backed documents share ONE page, because they share one WKWebView — a
    // WebContent process is ~187 MB, so a view per note caps the companion at about two of
    // them. This is the one place the pager's "a document is a page" rule bends, and it
    // bends because the platform says so, not because the design does.
    val webDocs = state.documents.documents.filterIsInstance<OpenDocument.Web>()

    // Created ABOVE the pager and unconditionally, so the view can never share a lifetime
    // with the slot showing it — the rule that survives a recycler landing later.
    // "Open in folder" on a board's selection bar arrives over the bridge as an artifact
    // id; opening it is the same event a Browser row's Open button sends.
    // "Ask about this" opens the copilot pane; seeding it with the object is for when the
    // companion's copilot exists (the pane is a placeholder today).
    val webHost = rememberWebSurfaceHost(
        state.editor,
        enabled = webDocs.isNotEmpty(),
        onOpenArtifact = { id, page ->
            // The wormhole: a cited page seeds the reader's seek BEFORE (or while) it is
            // open — ReaderSeeks is compose state, so an already-open reader jumps too.
            if (page != null) ReaderSeeks.request(id, page)
            state.nav.handle(NavEvent.OpenDoc(TabKey.Artifact(id)))
        },
        onAskAbout = { if (!state.chrome.copilotOpen) state.chrome.handle(ChromeEvent.ToggleCopilot) },
    )

    // Everything the shell can show, as one flat list of pages: the three destinations, the
    // browser, the editor if anything web-backed is open, then every document with a surface
    // of its own. There is no document layer and no destination layer with a condition
    // deciding which draws — there are just pages, and one of them is current.
    //
    // The browser page is ALWAYS here, whether or not a browser tab is open, because a
    // breadcrumb jump shows the browser surface without opening one. Making it conditional
    // would leave `activeIndex` at its no-match fallback of 0, silently rendering Home.
    val pages = remember(state.destinations, state.documents.documents) {
        state.destinations.map(ShellPage::Dest) +
            ShellPage.Browser +
            listOfNotNull(webDocs.takeIf { it.isNotEmpty() }?.let(ShellPage::Web)) +
            state.documents.documents.filterIsInstance<OpenDocument.Standalone>().map(ShellPage::Doc)
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
                Destination.Markets -> Placeholder("Markets", "Index strip, watchlist and positions.")
                Destination.Graph -> Placeholder("Graph", "The entity canvas.")
            }

            // The browser as a full surface — the promoted tab, and the breadcrumb jump.
            // Wider columns than the dropdown's, and its own footer.
            ShellPage.Browser -> Column(Modifier.fillMaxSize()) {
                Box(Modifier.fillMaxWidth().weight(1f)) {
                    BrowserRail(
                        slice = state.browser,
                        columnWidth = AnchorTheme.metrics.browserTabColumn,
                        onNav = state.nav.handle,
                    )
                }
                HorizontalHairline()
                BrowserFooter(state.browser, state.nav.handle, onOrganize = state.write.handle)
            }

            // Switching between two notes never moves the pager: they are the same page,
            // and the editor is told which pane to show. Nothing native is touched.
            is ShellPage.Web -> if (webHost == null) {
                Placeholder(page.docs.first().title, "Starting the editor…")
            } else {
                WebSurfacePage(
                    host = webHost,
                    panes = page.docs.map(OpenDocument.Web::asPane),
                    activeId = (state.documents.active as? OpenDocument.Web)?.key?.nodeId,
                )
            }

            is ShellPage.Doc -> DocumentPage(page.document)
        }
    }
}

/**
 * The editor's vocabulary, which is not quite the server's: it renders `page` and `shard`,
 * where the workspace stores `page` and `app`. One translation, at the boundary.
 */
private fun OpenDocument.Web.asPane(): WebPane = WebPane(
    rev = rev,
    id = key.nodeId,
    kind = when (kind) {
        ArtifactKind.App -> WebPane.Kind.Shard
        ArtifactKind.Board -> WebPane.Kind.Board
        else -> WebPane.Kind.Page
    },
    title = title,
)

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

    data class Doc(val document: OpenDocument.Standalone) : ShellPage {
        override val key: String get() = document.key.asString()
    }

    /**
     * The columns, full width — one page, always present.
     *
     * It stays resident for the same reason every page does, and it must exist even when no
     * browser tab is open: the breadcrumb's `Browser` crumb shows this surface without
     * promoting anything.
     */
    data object Browser : ShellPage {
        override val key: String get() = "browser"
    }

    /**
     * Every web-backed document as ONE page, because they share one web view.
     *
     * The key is a constant for the same reason it is a page: opening a second note must
     * not move the page already showing the first, and closing one must not renumber it.
     */
    data class Web(val docs: List<OpenDocument.Web>) : ShellPage {
        override val key: String get() = "web"
    }

    fun matches(surface: Surface): Boolean = when {
        this is Dest && surface is Surface.Dest -> destination == surface.destination
        this is Browser -> surface is Surface.Browser
        this is Web && surface is Surface.Doc -> docs.any { it.key == surface.key }
        this is Doc && surface is Surface.Doc -> document.key == surface.key
        else -> false
    }
}

/** One open document. The kind decides the surface; nothing here decides whether it exists. */
@Composable
private fun DocumentPage(document: OpenDocument.Standalone) {
    when (document) {
        is OpenDocument.Pdf -> when (val bytes = document.bytes) {
            // A genuine first open. Anything fetched before paints on frame one, because the
            // resource store answers from its cache synchronously.
            Fetch.Pending -> Placeholder(document.title, "Fetching…")
            // Said out loud rather than left spinning: a failure that looks like a slow
            // success is a surface that waits forever.
            is Fetch.Failed -> Placeholder(document.title, "Couldn't open it — ${bytes.reason}.")
            // The full reader: contents rail, chrome, position memory, and the wormhole's
            // ReaderSeeks — everything the bare PdfViewer (page 0, no re-seek) could not do.
            is Fetch.Ready -> DocumentReader(
                artifactId = document.key.nodeId,
                title = document.title,
                artifactType = document.artifactType,
                filePath = bytes.path,
                contentType = bytes.contentType,
                document = document.document,
                modifier = Modifier.fillMaxSize(),
                syncedPage = document.syncedPage,
                syncedAt = document.syncedAt,
                syncResolved = document.syncResolved,
                onPageViewed = document.onPageViewed,
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
