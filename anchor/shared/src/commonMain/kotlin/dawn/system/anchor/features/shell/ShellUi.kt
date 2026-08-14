package dawn.system.anchor.features.shell

import androidx.compose.animation.core.CubicBezierEasing
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
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
import androidx.compose.foundation.layout.requiredWidth
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import dawn.system.anchor.domain.ContentRow
import dawn.system.anchor.domain.DetailSection
import dawn.system.anchor.domain.Destination
import dawn.system.anchor.domain.StatCard
import dawn.system.anchor.domain.Tone
import dawn.system.anchor.domain.NodeKind
import dawn.system.anchor.domain.WorkspaceNode
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.ChipStyle
import dawn.system.anchor.services.design.MetaStyle
import dawn.system.anchor.services.design.SectionLabelStyle
import dawn.system.anchor.services.design.destinationIcon
import dawn.system.anchor.services.sync.SyncStatus

/**
 * The shell's chrome. Stateless: it renders [ShellScreen.State] and emits events, which is
 * what lets the same layout be driven by a test, a preview, or the real presenter.
 */
@Composable
fun ShellUi(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(modifier.fillMaxSize().background(c.bg)) {
        Rail(state)

        // Collapsing animates width to zero while the content keeps its natural width, so
        // text does not reflow mid-animation.
        val listWidth by animateDpAsState(
            targetValue = if (state.listOpen) m.listWidth else 0.dp,
            animationSpec = tween(
                durationMillis = dawn.system.anchor.services.design.AladinMetrics.COLUMN_COLLAPSE_MILLIS,
                easing = CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f),
            ),
            label = "list-collapse",
        )
        Box(Modifier.width(listWidth).fillMaxHeight().clipToWidth()) {
            ListColumn(state, Modifier.requiredWidth(m.listWidth))
        }

        DetailColumn(state, Modifier.weight(1f))
    }
}

private fun Modifier.clipToWidth(): Modifier = this.clip(RoundedCornerShape(0.dp))

@Composable
private fun Rail(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(
        Modifier
            .width(m.railWidth)
            .fillMaxHeight()
            .background(c.rail)
            .padding(vertical = 14.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        // Logo
        Box(
            Modifier
                .size(38.dp)
                .clip(AnchorShape.control)
                .background(c.amberSoft),
            contentAlignment = Alignment.Center,
        ) {
            Text("A", style = androidx.compose.material3.MaterialTheme.typography.titleMedium, color = c.amber)
        }
        Spacer(Modifier.height(18.dp))

        Destination.entries.forEach { destination ->
            RailCell(
                destination = destination,
                active = destination == state.destination,
                onClick = { state.eventSink(ShellScreen.Event.DestinationSelected(destination)) },
            )
            Spacer(Modifier.height(6.dp))
        }

        Spacer(Modifier.weight(1f))

        // Bottom cluster: open items with its count, then account.
        OpenItemsCell(
            count = state.openItems.size,
            onClick = { state.eventSink(ShellScreen.Event.ToggleSwitcher) },
        )
        Spacer(Modifier.height(10.dp))
        SyncDot(state.sync)
    }
}

@Composable
private fun RailCell(destination: Destination, active: Boolean, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Box(contentAlignment = Alignment.Center) {
        // The active marker bleeds off the left edge, as in the design.
        if (active) {
            Box(
                Modifier
                    .align(Alignment.CenterStart)
                    .size(width = 3.dp, height = 24.dp)
                    .background(c.amber),
            )
        }
        Box(
            Modifier
                .size(m.railCell)
                .clip(AnchorShape.railCell)
                .background(if (active) c.amberSoft else Color.Transparent)
                .clickable(onClick = onClick),
            contentAlignment = Alignment.Center,
        ) {
            destinationIcon(
                destination = destination,
                tint = if (active) c.amber else c.ink3,
                size = 22.dp,
            )
        }
    }
}

@Composable
private fun OpenItemsCell(count: Int, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Box {
        Box(
            Modifier
                .size(m.railCell)
                .clip(AnchorShape.railCell)
                .clickable(onClick = onClick),
            contentAlignment = Alignment.Center,
        ) {
            dawn.system.anchor.services.design.OpenItemsIcon(c.ink3, size = 22.dp)
        }
        if (count > 0) {
            Box(
                Modifier
                    .align(Alignment.TopEnd)
                    .size(18.dp)
                    .clip(AnchorShape.pill)
                    .background(c.amber),
                contentAlignment = Alignment.Center,
            ) {
                Text("$count", style = ChipStyle, color = c.onAmber)
            }
        }
    }
}

@Composable
private fun SyncDot(status: SyncStatus) {
    val c = AnchorTheme.colors
    val (tint, label) = when (status) {
        is SyncStatus.Synced -> c.forColor to "synced"
        is SyncStatus.Offline -> c.ink4 to "offline"
        SyncStatus.Idle -> c.ink4 to "idle"
    }
    Box(
        Modifier.size(38.dp).clip(AnchorShape.pill).background(c.field),
        contentAlignment = Alignment.Center,
    ) {
        Box(Modifier.size(8.dp).clip(AnchorShape.pill).background(tint))
    }
}

@Composable
private fun ListColumn(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors

    Column(modifier.fillMaxHeight().background(c.panel)) {
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                state.destination.title,
                style = androidx.compose.material3.MaterialTheme.typography.headlineMedium,
                color = c.ink,
                modifier = Modifier.weight(1f),
            )
            Text("${state.rows.size}", style = MetaStyle, color = c.ink4)
        }

        if (state.rows.isEmpty()) {
            Text(
                emptyCopy(state),
                style = MetaStyle,
                color = c.ink4,
                modifier = Modifier.padding(horizontal = 20.dp),
            )
        }

        LazyColumn(
            Modifier.fillMaxSize().padding(horizontal = 12.dp),
            verticalArrangement = Arrangement.spacedBy(AnchorTheme.space.rowGap),
        ) {
            items(state.rows, key = { it.id }) { node ->
                ListRow(
                    node = node,
                    selected = node.id == state.selected?.id,
                    onClick = { state.eventSink(ShellScreen.Event.RowSelected(node)) },
                )
            }
        }
    }
}

private fun emptyCopy(state: ShellScreen.State): String = when {
    state.sync is SyncStatus.Offline -> "offline — showing what was cached"
    state.destination == Destination.Folders -> "nothing synced yet"
    else -> "no list for this destination yet"
}

@Composable
private fun ListRow(node: WorkspaceNode, selected: Boolean, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier
            .fillMaxWidth()
            .heightIn(min = m.listRowMinHeight)
            .clip(AnchorShape.row)
            .background(if (selected) c.sel else Color.Transparent)
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier
                .size(9.dp)
                .clip(AnchorShape.pill)
                .background(if (node.isContainer) c.amber else c.ink3),
        )
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(
                node.title.ifBlank { "Untitled" },
                style = androidx.compose.material3.MaterialTheme.typography.titleMedium,
                color = if (selected) c.ink else c.ink2,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            val chip = chipFor(node)
            if (chip != null) {
                Spacer(Modifier.height(5.dp))
                Chip(chip, node.kind)
            }
        }
    }
}

@Composable
private fun Chip(label: String, kind: NodeKind) {
    val c = AnchorTheme.colors
    val (fg, bg) = when (kind) {
        NodeKind.Research -> c.amber to c.amberSoft
        NodeKind.Folder -> c.ink2 to c.hover
        NodeKind.Artifact -> c.ink2 to c.hover
    }
    Box(
        Modifier.clip(AnchorShape.chip).background(bg).padding(horizontal = 7.dp, vertical = 3.dp),
    ) {
        Text(label.uppercase(), style = ChipStyle, color = fg)
    }
}

/** A plain folder gets no chip — "folder" on a folder is noise (handoff). */
private fun chipFor(node: WorkspaceNode): String? = when (node.kind) {
    NodeKind.Research -> "research"
    NodeKind.Artifact -> node.artifactType
    NodeKind.Folder -> null
}

@Composable
private fun DetailColumn(state: ShellScreen.State, modifier: Modifier = Modifier) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Column(modifier.fillMaxHeight().background(c.bg)) {
        DetailHeader(state)
        Box(Modifier.fillMaxSize()) {
            val node = state.selected
            if (node == null) {
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    Text("Select something on the left", style = MetaStyle, color = c.ink4)
                }
            } else {
                DetailBody(state.sections)
            }
        }
    }
}

@Composable
private fun DetailHeader(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier.fillMaxWidth().height(m.detailHeaderHeight).padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        HeaderButton(
            onClick = { state.eventSink(ShellScreen.Event.ToggleList) },
            tint = if (state.listOpen) c.amber else c.ink3,
        ) {
            dawn.system.anchor.services.design.ColumnsIcon(
                if (state.listOpen) c.amber else c.ink3,
                size = 20.dp,
            )
        }
        Spacer(Modifier.width(10.dp))

        Column(Modifier.weight(1f)) {
            Text(
                state.selected?.title?.ifBlank { "Untitled" } ?: state.destination.title,
                style = androidx.compose.material3.MaterialTheme.typography.headlineSmall,
                color = c.ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val subtitle = state.selected?.let { subtitleFor(it) }
            if (subtitle != null) {
                Text(subtitle, style = MetaStyle, color = c.ink4, maxLines = 1)
            }
        }

        if (state.openItems.size > 0) {
            Stepper(state)
            Spacer(Modifier.width(8.dp))
            OpenPill(state)
        }
    }
}

@Composable
private fun Stepper(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    Row(
        Modifier.clip(AnchorShape.pill).background(c.field).padding(horizontal = 6.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        StepArrow("‹") { state.eventSink(ShellScreen.Event.StepOpenItem(-1)) }
        Text(
            "${state.openItems.activeOrdinal} of ${state.openItems.size}",
            style = MetaStyle,
            color = c.ink2,
            modifier = Modifier.padding(horizontal = 8.dp),
        )
        StepArrow("›") { state.eventSink(ShellScreen.Event.StepOpenItem(1)) }
    }
}

@Composable
private fun StepArrow(glyph: String, onClick: () -> Unit) {
    val c = AnchorTheme.colors
    Box(
        Modifier.size(28.dp).clip(AnchorShape.pill).clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Text(glyph, style = androidx.compose.material3.MaterialTheme.typography.titleMedium, color = c.ink3)
    }
}

@Composable
private fun OpenPill(state: ShellScreen.State) {
    val c = AnchorTheme.colors
    Row(
        Modifier
            .clip(AnchorShape.pill)
            .background(c.field)
            .clickable { state.eventSink(ShellScreen.Event.ToggleSwitcher) }
            .padding(horizontal = 14.dp, vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("Open", style = androidx.compose.material3.MaterialTheme.typography.labelLarge, color = c.ink)
        Spacer(Modifier.width(7.dp))
        Text("${state.openItems.size}", style = MetaStyle, color = c.ink4)
    }
}

private fun subtitleFor(node: WorkspaceNode): String = buildList {
    add(
        when (node.kind) {
            NodeKind.Research -> "research folder"
            NodeKind.Folder -> "folder"
            NodeKind.Artifact -> node.artifactType ?: "artifact"
        },
    )
    node.summary?.takeIf { it.isNotBlank() }?.let { add(it) }
}.joinToString(" · ")

@Composable
private fun HeaderButton(onClick: () -> Unit, tint: Color, content: @Composable () -> Unit) {
    Box(
        Modifier
            .size(AnchorTheme.metrics.touchTarget)
            .clip(AnchorShape.control)
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) { content() }
}


/**
 * One renderer for every destination's detail, driven by the section vocabulary. Adding a
 * destination adds *sections*, never a layout — which is the whole point of the vocabulary.
 */
@Composable
private fun DetailBody(sections: List<DetailSection>) {
    val c = AnchorTheme.colors

    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 24.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        sections.forEach { section ->
            when (section) {
                is DetailSection.Stats -> StatsRow(section.cards)
                is DetailSection.Prose -> ProseCard(section.label, section.statement)
                is DetailSection.Contents -> ContentsList(section.label, section.items)
                is DetailSection.Note -> Text(section.text, style = MetaStyle, color = c.ink4)
                is DetailSection.Topics -> SectionLabel(section.label)
            }
        }
        Spacer(Modifier.height(28.dp))
    }
}

@Composable
private fun SectionLabel(label: String) {
    Text(label.uppercase(), style = SectionLabelStyle, color = AnchorTheme.colors.ink4)
}

@Composable
private fun StatsRow(cards: List<StatCard>) {
    val c = AnchorTheme.colors
    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        cards.forEach { card ->
            Column(
                Modifier
                    .weight(1f)
                    .clip(AnchorShape.card)
                    .background(c.card)
                    .padding(horizontal = 18.dp, vertical = 16.dp),
            ) {
                Text(card.label.uppercase(), style = SectionLabelStyle, color = c.ink4)
                Spacer(Modifier.height(8.dp))
                Text(
                    card.value,
                    style = androidx.compose.material3.MaterialTheme.typography.titleLarge,
                    color = when (card.tone) {
                        Tone.Amber -> c.amber
                        Tone.For -> c.forColor
                        Tone.Against -> c.against
                        Tone.Ink -> c.ink
                    },
                )
            }
        }
    }
}

@Composable
private fun ProseCard(label: String, statement: String) {
    val c = AnchorTheme.colors
    Column(Modifier.fillMaxWidth()) {
        SectionLabel(label)
        Spacer(Modifier.height(8.dp))
        Box(
            Modifier
                .fillMaxWidth()
                .clip(AnchorShape.card)
                .background(c.card)
                .padding(20.dp),
        ) {
            Text(
                statement,
                style = androidx.compose.material3.MaterialTheme.typography.bodyLarge,
                color = c.ink2,
            )
        }
    }
}

@Composable
private fun ContentsList(label: String, items: List<ContentRow>) {
    val c = AnchorTheme.colors
    Column(Modifier.fillMaxWidth()) {
        SectionLabel(label)
        Spacer(Modifier.height(8.dp))
        items.forEach { row ->
            Row(
                Modifier
                    .fillMaxWidth()
                    .clip(AnchorShape.row)
                    .background(c.card)
                    .padding(horizontal = 18.dp, vertical = 15.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    Modifier
                        .size(8.dp)
                        .clip(AnchorShape.pill)
                        .background(if (row.kind == NodeKind.Artifact) c.ink3 else c.amber),
                )
                Spacer(Modifier.width(12.dp))
                Text(
                    row.title,
                    style = androidx.compose.material3.MaterialTheme.typography.titleMedium,
                    color = c.ink2,
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                row.meta?.let { Text(it, style = MetaStyle, color = c.ink4) }
            }
            Spacer(Modifier.height(8.dp))
        }
    }
}
