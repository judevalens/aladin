package com.jvp.aladin_compose.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.ExperimentalComposeUiApi
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.WebElementView
import kotlinx.browser.document
import org.w3c.dom.HTMLDivElement
import org.w3c.dom.HTMLElement

@JsName("AladinBridge")
external object AladinBridge {
    fun mountDemo(
        element: HTMLElement,
        onTick: (Int) -> Unit,
        onButtonPressed: () -> Unit,
    )

    fun updateDemo(element: HTMLElement, message: String)

    fun disposeDemo(element: HTMLElement)
}

@OptIn(ExperimentalComposeUiApi::class)
@Composable
fun WebInteropTestScreen() {
    var message by remember { mutableStateOf("Hello from Kotlin") }
    var tickCount by remember { mutableStateOf(0) }
    var buttonCount by remember { mutableStateOf(0) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
            Text(
                "WebElementView Direct Callback",
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                "The DOM island is mounted by an external JS file. JS calls Kotlin directly through callbacks passed during mount.",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }

        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(onClick = { message = "Message updated by Kotlin (${buttonCount + 1})" }) {
                Text("Push State To JS")
            }
            Button(onClick = {
                tickCount = 0
                buttonCount = 0
                message = "Reset from Kotlin"
            }) {
                Text("Reset Counters")
            }
        }

        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            StatusCard("JS interval ticks", tickCount.toString())
            StatusCard("JS button callbacks", buttonCount.toString())
        }

        WebElementView(
            modifier = Modifier
                .fillMaxWidth()
                .height(360.dp)
                .clip(RoundedCornerShape(20.dp))
                .border(1.dp, MaterialTheme.colorScheme.outlineVariant, RoundedCornerShape(20.dp)),
            factory = {
                (document.createElement("div") as HTMLDivElement).apply {
                    style.width = "100%"
                    style.height = "100%"
                    AladinBridge.mountDemo(
                        element = this,
                        onTick = { tickCount = it },
                        onButtonPressed = { buttonCount += 1 }
                    )
                    AladinBridge.updateDemo(this, message)
                }
            },
            update = { element ->
                AladinBridge.updateDemo(element, message)
            },
            onRelease = { element ->
                AladinBridge.disposeDemo(element)
            }
        )

        Row(
            modifier = Modifier
                .clip(RoundedCornerShape(14.dp))
                .background(Color(0xFF101826))
                .padding(horizontal = 14.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            Box(
                modifier = Modifier
                    .size(10.dp)
                    .clip(RoundedCornerShape(99.dp))
                    .background(Color(0xFF38BDF8))
            )
            Text(
                "Kotlin owns the state. The external JS file owns the DOM subtree and calls Kotlin through the provided callbacks.",
                style = MaterialTheme.typography.bodySmall,
                color = Color.White
            )
        }
    }
}

@Composable
private fun StatusCard(label: String, value: String) {
    Column(
        modifier = Modifier
            .clip(RoundedCornerShape(14.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(4.dp)
    ) {
        Text(
            label,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            value,
            style = MaterialTheme.typography.titleLarge,
            color = MaterialTheme.colorScheme.onSurface
        )
    }
}
