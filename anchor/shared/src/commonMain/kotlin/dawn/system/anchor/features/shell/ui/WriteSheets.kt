package dawn.system.anchor.features.shell.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.unit.dp
import dawn.system.anchor.features.shell.ShellScreen
import dawn.system.anchor.features.shell.state.WriteEvent
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme
import dawn.system.anchor.services.design.SectionLabelStyle

/**
 * Rename and delete, as the two sheets the browser tab's organize footer raises.
 *
 * These are **modal on purpose**, unlike the browser dropdown: a rename is a question with an
 * answer, and a delete is a decision. The dropdown is undimmed because your document must stay
 * legible behind it; there is nothing to keep legible behind a confirmation.
 *
 * Composed at the shell's box for the same reason the overlays are: a sheet inside a pager page
 * would park with it.
 */
@Composable
internal fun BoxScope.WriteSheets(state: ShellScreen.State) {
    val write = state.write

    write.rename?.let { draft ->
        Sheet(onDismiss = { write.handle(WriteEvent.RenameCancelled) }) {
            val c = AnchorTheme.colors
            Text("RENAME", style = SectionLabelStyle, color = c.ink4)
            Spacer(Modifier.height(10.dp))
            BasicTextField(
                value = draft.title,
                onValueChange = { write.handle(WriteEvent.RenameEdited(it)) },
                singleLine = true,
                textStyle = TextStyle(
                    color = c.ink,
                    fontSize = MaterialTheme.typography.titleMedium.fontSize,
                ),
                cursorBrush = androidx.compose.ui.graphics.SolidColor(c.amber),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(AnchorTheme.metrics.touchTarget)
                    .clip(AnchorShape.field)
                    .background(c.field)
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            )
            Spacer(Modifier.height(16.dp))
            Actions(
                confirm = "Save",
                confirmEnabled = draft.title.isNotBlank() && !draft.saving,
                onConfirm = { write.handle(WriteEvent.RenameCommitted) },
                onCancel = { write.handle(WriteEvent.RenameCancelled) },
            )
        }
    }

    write.pendingDelete?.let { node ->
        Sheet(onDismiss = { write.handle(WriteEvent.DeleteCancelled) }) {
            val c = AnchorTheme.colors
            Text("DELETE", style = SectionLabelStyle, color = c.ink4)
            Spacer(Modifier.height(10.dp))
            Text(
                node.title.takeIf { it.isNotBlank() } ?: "Untitled",
                style = MaterialTheme.typography.titleMedium,
                color = c.ink,
            )
            Spacer(Modifier.height(6.dp))
            Text(
                if (node.isContainer) {
                    "Everything inside it goes too."
                } else {
                    "This cannot be undone."
                },
                style = MaterialTheme.typography.bodyMedium,
                color = c.ink3,
            )
            Spacer(Modifier.height(16.dp))
            Actions(
                confirm = "Delete",
                destructive = true,
                onConfirm = { write.handle(WriteEvent.DeleteConfirmed) },
                onCancel = { write.handle(WriteEvent.DeleteCancelled) },
            )
        }
    }

    // A write that failed says so. Silence here would look like a rename that simply did not
    // take, which is indistinguishable from one the server rejected.
    write.writeError?.let { message ->
        Sheet(onDismiss = { write.handle(WriteEvent.WriteErrorDismissed) }) {
            val c = AnchorTheme.colors
            Text("COULDN'T SAVE", style = SectionLabelStyle, color = c.against)
            Spacer(Modifier.height(10.dp))
            Text(message, style = MaterialTheme.typography.bodyMedium, color = c.ink2)
            Spacer(Modifier.height(16.dp))
            Actions(
                confirm = "OK",
                onConfirm = { write.handle(WriteEvent.WriteErrorDismissed) },
                onCancel = null,
            )
        }
    }
}

@Composable
private fun BoxScope.Sheet(onDismiss: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    val c = AnchorTheme.colors
    val interaction = remember { MutableInteractionSource() }

    Box(
        Modifier
            .fillMaxSize()
            .background(c.bg.copy(alpha = 0.55f))
            .clickable(interaction, indication = null, onClick = onDismiss),
    )
    Column(
        Modifier
            .align(Alignment.Center)
            .width(420.dp)
            .shadow(24.dp, AnchorShape.popover)
            .clip(AnchorShape.popover)
            .background(c.card)
            .padding(22.dp),
        content = content,
    )
}

@Composable
private fun Actions(
    confirm: String,
    onConfirm: () -> Unit,
    onCancel: (() -> Unit)?,
    confirmEnabled: Boolean = true,
    destructive: Boolean = false,
) {
    val c = AnchorTheme.colors
    val m = AnchorTheme.metrics

    Row(
        Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        onCancel?.let {
            Row(
                Modifier
                    .height(m.touchTarget)
                    .clip(AnchorShape.row)
                    .clickable(onClick = it)
                    .padding(horizontal = 16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Cancel", style = MaterialTheme.typography.titleMedium, color = c.ink3)
            }
        }
        Row(
            Modifier
                .height(m.touchTarget)
                .clip(AnchorShape.row)
                .background(if (destructive) c.against else c.amber)
                .clickable(enabled = confirmEnabled, onClick = onConfirm)
                .padding(horizontal = 20.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(confirm, style = MaterialTheme.typography.titleMedium, color = c.onAmber)
        }
    }
}
