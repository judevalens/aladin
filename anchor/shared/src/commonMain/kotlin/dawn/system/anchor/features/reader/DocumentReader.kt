package dawn.system.anchor.features.reader

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import dawn.system.anchor.domain.DocumentStatus
import dawn.system.anchor.domain.IngestedDocument
import dawn.system.anchor.domain.OutlineEntry
import dawn.system.anchor.domain.OutlineSource
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.data.ArtifactResourceStore
import dawn.system.anchor.services.data.CachedResource
import dawn.system.anchor.services.data.DocumentStore
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChevronDirection
import dawn.system.anchor.services.design.ChevronIcon
import dawn.system.anchor.services.design.ChipStyle
import dawn.system.anchor.services.design.FileGlyphSize
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.design.MinusIcon
import dawn.system.anchor.services.design.PlusIcon
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.SidebarIcon
import dawn.system.anchor.services.design.artifactKindIcon
import dawn.system.anchor.services.platform.NativePdfReader

/**
 * A `file` artifact — the PDF reader (VIEWERS.md §1).
 *
 * **It shows the page, not the extracted text.** That desktop decision carries over
 * verbatim: the extracted layer is for machines, and the outline is navigation *over* the
 * real page rather than a substitute for it.
 *
 * Three parts, left to right: a contents rail, a chrome bar, and the page stage. The rail
 * and the chrome are Compose; the stage is PDFKit, which gives real text selection, search
 * and tiled rendering of a 400-page document for free.
 */
@Composable
fun DocumentReader(
    node: WorkspaceNode,
    resources: ArtifactResourceStore,
    documents: DocumentStore,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors
    // Seeded from the cache SYNCHRONOUSLY. Starting at Loading and resolving in an effect
    // paints a full-pane "Fetching…" over a file that is already on disk — one frame, but a
    // frame on every fresh composition, which is exactly what switching pages causes.
    var state by remember(node.id) {
        mutableStateOf<ReaderState>(
            resources.cached(node.id)?.let(ReaderState::Ready) ?: ReaderState.Loading,
        )
    }
    var document by remember(node.id) { mutableStateOf<IngestedDocument?>(null) }

    // This reader is a RECYCLED surface: it is unmounted as soon as you look at something
    // else, so everything worth keeping has to live outside the composition. That snapshot
    // is the whole contract — reopening a 300-page paper at page 1 is the fastest way to
    // make a reader useless, and it is what holding the view resident used to buy.
    val restored = remember(node.id) { ReaderPositions.of(node.id) }
    var page by remember(node.id) { mutableStateOf(restored.page) }
    var pageCount by remember(node.id) { mutableStateOf(0) }
    var zoom by remember(node.id) { mutableStateOf(restored.zoom) }
    var railOpen by remember(node.id) { mutableStateOf(restored.railOpen) }

    LaunchedEffect(node.id) {
        if (state is ReaderState.Ready) return@LaunchedEffect // already on disk
        state = try {
            ReaderState.Ready(resources.resource(node.id))
        } catch (failure: Throwable) {
            ReaderState.Failed(failure.message ?: "could not fetch the file")
        }
    }

    // The ingested half is fetched alongside, not before: a PDF with no outline still
    // reads, so the stage must never wait on this.
    LaunchedEffect(node.id) {
        document = runCatching { documents.document(node.id) }.getOrNull()
    }

    LaunchedEffect(node.id, page, zoom, railOpen) {
        ReaderPositions.remember(node.id, ReaderPosition(page, zoom, railOpen))
    }

    val current = state
    val status = document?.status ?: DocumentStatus.Ready

    Box(modifier.fillMaxSize().background(c.rail)) {
        when {
            current is ReaderState.Loading -> ReaderNotice(
                title = "Fetching…",
                detail = "Downloading ${node.title.ifBlank { "the file" }}.",
            )

            current is ReaderState.Failed -> ReaderNotice(
                title = "Couldn't load the file",
                detail = current.message,
            )

            current is ReaderState.Ready && !current.resource.isPdf ->
                UnsupportedFileCard(node, current.resource)

            // A stalled or failed ingestion hides the chrome entirely: an outline rail
            // over a document that has no outline yet is furniture, not navigation.
            !status.isReadable -> StatusNotice(status, document?.error)

            current is ReaderState.Ready -> Row(Modifier.fillMaxSize()) {
                if (railOpen) {
                    ContentsRail(
                        document = document,
                        page = page,
                        onJump = { page = it },
                    )
                    Box(Modifier.width(1.dp).fillMaxHeight().background(c.line))
                }

                Column(Modifier.weight(1f).fillMaxHeight()) {
                    ReaderChrome(
                        railOpen = railOpen,
                        onToggleRail = { railOpen = !railOpen },
                        section = document?.entryAt(page)?.title,
                        page = page,
                        pageCount = if (pageCount > 0) pageCount else document?.pageCount ?: 0,
                        zoom = zoom,
                        onPage = { page = it.coerceIn(1, maxOf(pageCount, 1)) },
                        onZoom = { zoom = it },
                    )
                    NativePdfReader(
                        filePath = current.resource.path,
                        page = page,
                        zoom = zoom,
                        onDocumentLoaded = { pageCount = it },
                        onPageChanged = { page = it },
                        modifier = Modifier.fillMaxSize(),
                    )
                }
            }
        }
    }
}

/** Everything the reader needs to come back looking exactly as you left it. */
private data class ReaderPosition(
    val page: Int = 1,
    val zoom: Float = 1f,
    /** Whether the contents rail was open — a preference, and one you notice losing. */
    val railOpen: Boolean = true,
)

/**
 * The snapshot store for recycled readers.
 *
 * Process-scoped and in memory: this exists so unmounting a reader is free, not to survive
 * a relaunch. Persisting across launches is a separate, larger question — losing your place
 * mid-session is the one that actually bites.
 */
private object ReaderPositions {
    private val byArtifact = mutableMapOf<String, ReaderPosition>()

    fun remember(artifactId: String, position: ReaderPosition) {
        byArtifact[artifactId] = position
    }

    fun of(artifactId: String): ReaderPosition = byArtifact[artifactId] ?: ReaderPosition()
}

/**
 * The contents rail. Its active row is **a position, not a link you clicked**: the last
 * entry starting at or before the current page, so scrolling through the middle of a
 * section keeps that section lit.
 */
@Composable
private fun ContentsRail(document: IngestedDocument?, page: Int, onJump: (Int) -> Unit) {
    val c = AnchorTheme.colors
    val outline = document?.outline.orEmpty()
    val activePage = document?.entryAt(page)?.page
    val listState = rememberLazyListState()

    // Follow the reader: an active row scrolled out of view says nothing.
    LaunchedEffect(activePage) {
        outline.indexOfFirst { it.page == activePage }
            .takeIf { it >= 0 }
            ?.let { listState.animateScrollToItem(it) }
    }

    Column(Modifier.width(RAIL_WIDTH).fillMaxHeight().background(c.panel)) {
        Row(
            Modifier.fillMaxWidth().padding(start = 18.dp, end = 12.dp, top = 15.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text("Contents".uppercase(), style = SectionLabelStyle, color = c.ink4)
            Text(outline.size.toString(), style = MetaStyle, color = c.ink4, modifier = Modifier.weight(1f))
            // Only when the outline was inferred. An authored one shows nothing, because a
            // file's own bookmarks are not a claim that needs qualifying.
            if (document?.outlineSource == OutlineSource.Recovered) {
                Text(
                    "recovered",
                    style = ChipStyle,
                    color = c.amber,
                    modifier = Modifier
                        .clip(AnchorShape.chip)
                        .background(c.amberSoft)
                        .padding(horizontal = 7.dp, vertical = 3.dp),
                )
            }
        }

        if (outline.isEmpty()) {
            Text(
                "This file carries no outline.",
                style = MetaStyle,
                color = c.ink4,
                modifier = Modifier.padding(horizontal = 18.dp, vertical = 6.dp),
            )
        } else {
            LazyColumn(
                Modifier.fillMaxWidth().weight(1f).padding(horizontal = 10.dp),
                state = listState,
            ) {
                items(outline) { entry ->
                    OutlineRow(entry, active = entry.page == activePage) { onJump(entry.page) }
                }
            }
        }
    }
}

@Composable
private fun OutlineRow(entry: OutlineEntry, active: Boolean, onClick: () -> Unit) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = 44.dp)
            .clip(AnchorShape.chip)
            .background(if (active) c.amberSoft else Color.Transparent)
            .clickable(onClick = onClick)
            .padding(
                start = (10 + entry.depth * 16).dp,
                end = 10.dp,
                top = 11.dp,
                bottom = 11.dp,
            ),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Text(
            entry.title,
            style = MaterialTheme.typography.bodyMedium,
            color = if (active) c.amber else c.ink2,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f),
        )
        Text(entry.page.toString(), style = MetaStyle, color = if (active) c.amber else c.ink4)
    }
}

@Composable
private fun ReaderChrome(
    railOpen: Boolean,
    onToggleRail: () -> Unit,
    section: String?,
    page: Int,
    pageCount: Int,
    zoom: Float,
    onPage: (Int) -> Unit,
    onZoom: (Float) -> Unit,
) {
    val c = AnchorTheme.colors

    Row(
        Modifier
            .fillMaxWidth()
            .height(56.dp)
            .background(c.panel)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        ChromeButton(onClick = onToggleRail, pressed = railOpen) {
            SidebarIcon(if (railOpen) c.amber else c.ink3, size = 19.dp)
        }
        Text(
            section.orEmpty(),
            style = MetaStyle,
            color = c.ink4,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f).padding(horizontal = 6.dp),
        )

        ChromeButton(onClick = { onPage(page - 1) }, enabled = page > 1) {
            ChevronIcon(if (page > 1) c.ink3 else c.ink4, size = 18.dp, direction = ChevronDirection.Left)
        }
        Text(
            if (pageCount > 0) "$page / $pageCount" else "$page",
            style = MetaStyle,
            color = c.ink3,
            maxLines = 1,
        )
        ChromeButton(onClick = { onPage(page + 1) }, enabled = pageCount == 0 || page < pageCount) {
            ChevronIcon(c.ink3, size = 18.dp)
        }

        Box(Modifier.width(1.dp).height(24.dp).padding(horizontal = 0.dp).background(c.line))

        ChromeButton(
            onClick = { onZoom((zoom - ZOOM_STEP).coerceAtLeast(ZOOM_MIN)) },
            enabled = zoom > ZOOM_MIN,
        ) {
            MinusIcon(c.ink3, size = 16.dp)
        }
        Text(
            "${(zoom * 100).toInt()}%",
            style = MetaStyle,
            color = c.ink3,
            textAlign = TextAlign.Center,
            modifier = Modifier.widthIn(min = 42.dp),
        )
        ChromeButton(
            onClick = { onZoom((zoom + ZOOM_STEP).coerceAtMost(ZOOM_MAX)) },
            enabled = zoom < ZOOM_MAX,
        ) {
            PlusIcon(c.ink3, size = 16.dp)
        }
    }
    Box(Modifier.fillMaxWidth().height(1.dp).background(c.line))
}

@Composable
private fun ChromeButton(
    onClick: () -> Unit,
    enabled: Boolean = true,
    pressed: Boolean = false,
    content: @Composable () -> Unit,
) {
    Box(
        Modifier
            .size(44.dp)
            .clip(AnchorShape.control)
            .background(if (pressed) AnchorTheme.colors.sel else Color.Transparent)
            .clickable(enabled = enabled, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        content()
    }
}

/**
 * What happened to the ingestion, and what to do about it.
 *
 * Every state names an action, because the alternative — a status word on its own — leaves
 * the user with a file they can see is broken and no idea whose problem it is.
 */
@Composable
private fun StatusNotice(status: DocumentStatus, error: String?) {
    val (title, detail) = when (status) {
        DocumentStatus.Pending -> "Queued" to
            "This file is waiting for the ingestion worker to pick it up."

        DocumentStatus.Ingesting -> "Reading…" to
            "The worker is extracting this document's text and structure."

        DocumentStatus.Unsupported -> "No text to extract" to
            "This is almost certainly a scan. Run it through OCR and re-upload it from " +
                "the desktop app."

        DocumentStatus.Failed -> "Couldn't read this file" to
            (error ?: "Extraction failed. Retry it from the desktop app.")

        DocumentStatus.Ready -> "Ready" to ""
    }
    ReaderNotice(title = title, detail = detail)
}

private sealed interface ReaderState {
    data object Loading : ReaderState
    data class Ready(val resource: CachedResource) : ReaderState
    data class Failed(val message: String) : ReaderState
}

/**
 * A file the companion cannot render. It says what it is and where it went, rather than
 * pretending: the file *is* downloaded by this point, so the honest thing is to name it.
 */
@Composable
private fun UnsupportedFileCard(node: WorkspaceNode, resource: CachedResource) {
    val c = AnchorTheme.colors

    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.Center,
    ) {
        Row(
            Modifier
                .fillMaxWidth()
                .clip(AnchorShape.card)
                .background(c.card)
                .padding(20.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            artifactKindIcon(
                artifactType = node.artifactType,
                isContainer = false,
                tint = c.ink3,
                size = FileGlyphSize,
            )
            Column(Modifier.weight(1f)) {
                Text(
                    node.title.ifBlank { "Untitled file" },
                    style = MaterialTheme.typography.titleMedium,
                    color = c.ink,
                )
                Spacer(Modifier.height(4.dp))
                Text(resource.contentType ?: "unknown type", style = MetaStyle, color = c.ink4)
            }
        }
        Spacer(Modifier.height(12.dp))
        Text(
            "The companion reads PDFs. This one is downloaded but has no viewer here yet.",
            style = MetaStyle,
            color = c.ink4,
        )
    }
}

@Composable
private fun ReaderNotice(title: String, detail: String) {
    val c = AnchorTheme.colors
    Column(
        Modifier.fillMaxSize().padding(horizontal = 32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, style = MaterialTheme.typography.headlineSmall, color = c.ink)
        Spacer(Modifier.height(8.dp))
        Text(
            detail,
            style = MetaStyle,
            color = c.ink4,
            textAlign = TextAlign.Center,
            modifier = Modifier.widthIn(max = 440.dp),
        )
    }
}

private val RAIL_WIDTH = 292.dp
private const val ZOOM_MIN = 0.7f
private const val ZOOM_MAX = 1.45f
private const val ZOOM_STEP = 0.15f
