package dawn.system.anchor.services.design

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.unit.dp
import dawn.system.anchor.services.design.AnchorShape
import dawn.system.anchor.services.design.AnchorTheme

/**
 * Reference components showing how the tokens compose into Anchor's look.
 * Not exhaustive — a starting point that mirrors the prototype's card + accent patterns.
 */

/** Standard content card: surface fill, hairline border, soft rounding. */
@Composable
fun AnchorCard(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    val c = AnchorTheme.colors
    Column(
        modifier
            .clip(AnchorShape.card)
            .background(c.card)
            .border(1.dp, c.line2, AnchorShape.card)
            .padding(17.dp),
        content = content,
    )
}

/** Uppercase, letter-spaced section overline. */
@Composable
fun AnchorSectionLabel(text: String, modifier: Modifier = Modifier) {
    Text(
        text.uppercase(),
        modifier = modifier,
        color = AnchorTheme.colors.ink4,
        style = MaterialTheme.typography.labelSmall,
    )
}

/** The living "Anchor is speaking" dot — pair with reflective/coaching copy. */
@Composable
fun AnchorPresenceDot(modifier: Modifier = Modifier) {
    Box(
        modifier
            .size(7.dp)
            .clip(CircleShape)
            .background(AnchorTheme.colors.amber),
    )
}

/** Anchor's reflective card — raised surface, presence dot + label, then the read. */
@Composable
fun AnchorReflectionCard(
    label: String,
    body: String,
    modifier: Modifier = Modifier,
) {
    val c = AnchorTheme.colors
    Column(
        modifier
            .clip(AnchorShape.card)
            .background(c.raise)
            .border(1.dp, c.line, AnchorShape.card)
            .padding(18.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            AnchorPresenceDot()
            Spacer(Modifier.width(9.dp))
            AnchorSectionLabel(label)
        }
        Spacer(Modifier.height(11.dp))
        Text(
            body,
            color = c.ink2,
            style = MaterialTheme.typography.bodyLarge,
        )
    }
}
