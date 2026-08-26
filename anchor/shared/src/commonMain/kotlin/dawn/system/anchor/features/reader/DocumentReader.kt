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
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.debounce
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.runtime.mutableStateMapOf
import dawn.system.anchor.domain.IngestedDocument
import dawn.system.anchor.domain.OutlineEntry
import dawn.system.anchor.domain.OutlineSource
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
 *
 * **Value-driven** (adopted into the rev-2 shell, 2026-08-25): the bytes and the ingested
 * half arrive as values from DocumentStateProducer — no service reaches this layer. A PDF
 * always renders, ingested or not; the rail simply has nothing to show until the outline
 * lands. Reading position, zoom and the rail survive recycling via [ReaderPositions], and
 * the study loop's wormhole lands through [ReaderSeeks].
 */
@OptIn(FlowPreview::class)
@Composable
fun DocumentReader(
    artifactId: String,
    title: String,
    artifactType: String?,
    filePath: String,
    contentType: String?,
    document: IngestedDocument?,
    modifier: Modifier = Modifier,
    /** The synced cross-device position (page, server unix-ms stamp) — null = none known. */
    syncedPage: Int? = null,
    syncedAt: Long = 0,
    /** True once the synced position has been LOOKED UP (present or absent) — gates both
     *  the apply-at-open decision and the reporter, so the mount-time page can't win races. */
    syncResolved: Boolean = false,
    /** The (already debounce-tolerant) position report sink; null = don't report. */
    onPageViewed: ((Int) -> Unit)? = null,
) {
    val c = AnchorTheme.colors

    // This reader is a RECYCLED surface: it is unmounted as soon as you look at something
    // else, so everything worth keeping has to live outside the composition. That snapshot
    // is the whole contract — reopening a 300-page paper at page 1 is the fastest way to
    // make a reader useless.
    //
    // `sessionAtOpen` is the entry as it stood BEFORE this mount starts re-stamping it —
    // the newer-of comparison against the synced position must not race the persist effect
    // below, which would otherwise make the session look freshly written at mount.
    val sessionAtOpen = remember(artifactId) { ReaderPositions.entry(artifactId) }
    val restored = remember(artifactId) { ReaderPositions.of(artifactId) }
    var page by remember(artifactId) { mutableStateOf(restored.page) }
    var pageCount by remember(artifactId) { mutableStateOf(0) }
    var zoom by remember(artifactId) { mutableStateOf(restored.zoom) }
    var railOpen by remember(artifactId) { mutableStateOf(restored.railOpen) }

    LaunchedEffect(artifactId, page, zoom, railOpen) {
        ReaderPositions.remember(artifactId, ReaderPosition(page, zoom, railOpen, at = nowMs()))
    }

    // APPLY-AT-OPEN: once the synced position resolves, land on newer-of(session, synced) —
    // exactly once, so a frame arriving later never yanks the page mid-read. An explicit
    // seek (the effect below) always outranks the restore; page-1 positions aren't worth a
    // jump. `lastReported` doubles as the reporter's baseline: the restored page itself is
    // never reported back (the page-1-at-mount stomp, see the desktop's position-reporter).
    var lastReported by remember(artifactId) { mutableStateOf<Int?>(null) }
    LaunchedEffect(artifactId, syncResolved) {
        if (!syncResolved || lastReported != null) return@LaunchedEffect
        val sessionAt = sessionAtOpen?.at ?: 0L
        if (
            ReaderSeeks.current(artifactId) == null &&
            syncedPage != null && syncedPage > 1 &&
            (sessionAtOpen == null || syncedAt > sessionAt)
        ) {
            page = syncedPage
        }
        lastReported = page
    }

    // The reporter: real page changes (settled for a beat) flow out; the value the reader
    // opened on does not. PDFKit's onPageChanged fires per page while a long seek settles —
    // the debounce absorbs that. Leaving the reader flushes a still-pending change.
    LaunchedEffect(artifactId, lastReported != null) {
        if (lastReported == null || onPageViewed == null) return@LaunchedEffect
        snapshotFlow { page }
            .debounce(REPORT_DEBOUNCE_MILLIS)
            .collect { p ->
                if (p != lastReported) {
                    lastReported = p
                    onPageViewed(p)
                }
            }
    }
    DisposableEffect(artifactId) {
        onDispose {
            val last = lastReported
            if (onPageViewed != null && last != null && page != last) onPageViewed(page)
        }
    }

    // The wormhole: a cite (board excerpt, worksheet header) asked this document to open
    // at a page. Nonce-keyed so citing the same page twice still jumps.
    val seek = ReaderSeeks.current(artifactId)
    LaunchedEffect(seek?.nonce) {
        seek?.let { page = it.page.coerceAtLeast(1) }
    }

    val isPdf = contentType?.startsWith("application/pdf") == true ||
        filePath.endsWith(".pdf", ignoreCase = true) ||
        contentType == null // benefit of the doubt: PDFKit fails loudly, a card hides quietly

    Box(modifier.fillMaxSize().background(c.rail)) {
        when {
            !isPdf -> UnsupportedFileCard(title, artifactType, contentType)

            else -> Row(Modifier.fillMaxSize()) {
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
                        filePath = filePath,
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
    /** Device wall-clock ms of the last update — compared against the synced position's
     *  server stamp at open (newer-of). Zero on the default entry. */
    val at: Long = 0,
)

/**
 * The snapshot store for recycled readers.
 *
 * Process-scoped and in memory: this exists so unmounting a reader is free. Surviving a
 * relaunch — and a different device — is the synced `reading_position` row's job; at open
 * the reader takes whichever of the two is newer. Zoom and the rail stay session-local:
 * they are per-device preferences the sync deliberately does not carry.
 */
private object ReaderPositions {
    private val byArtifact = mutableMapOf<String, ReaderPosition>()

    fun remember(artifactId: String, position: ReaderPosition) {
        byArtifact[artifactId] = position
    }

    fun of(artifactId: String): ReaderPosition = byArtifact[artifactId] ?: ReaderPosition()

    /** The raw entry, or null if this document was never open this session. */
    fun entry(artifactId: String): ReaderPosition? = byArtifact[artifactId]
}

@OptIn(kotlin.time.ExperimentalTime::class)
private fun nowMs(): Long = kotlin.time.Clock.System.now().toEpochMilliseconds()

private const val REPORT_DEBOUNCE_MILLIS = 2_000L

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
 * The wormhole's landing pad: per-artifact "open at this page" requests, written by the
 * bridge (a board excerpt's Open source, a worksheet's cite) and observed by whichever
 * reader shows that document. A compose state map, so an ALREADY-OPEN reader jumps too.
 * Nonce-keyed so the same page re-fires. The iPad twin of the desktop's
 * `pendingDocLocations`.
 */
object ReaderSeeks {
    data class Seek(val page: Int, val nonce: Int)

    private val requests = mutableStateMapOf<String, Seek>()

    fun request(artifactId: String, page: Int) {
        val nonce = (requests[artifactId]?.nonce ?: 0) + 1
        requests[artifactId] = Seek(page, nonce)
    }

    /** Read inside composition — the reader recomposes when its document's seek changes. */
    fun current(artifactId: String): Seek? = requests[artifactId]
}

/**
 * A file the companion cannot render. It says what it is and where it went, rather than
 * pretending: the file *is* downloaded by this point, so the honest thing is to name it.
 */
@Composable
private fun UnsupportedFileCard(title: String, artifactType: String?, contentType: String?) {
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
                artifactType = artifactType,
                isContainer = false,
                tint = c.ink3,
                size = FileGlyphSize,
            )
            Column(Modifier.weight(1f)) {
                Text(
                    title.ifBlank { "Untitled file" },
                    style = MaterialTheme.typography.titleMedium,
                    color = c.ink,
                )
                Spacer(Modifier.height(4.dp))
                Text(contentType ?: "unknown type", style = MetaStyle, color = c.ink4)
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
