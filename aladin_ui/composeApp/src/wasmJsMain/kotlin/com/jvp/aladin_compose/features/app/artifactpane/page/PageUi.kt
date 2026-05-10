package com.jvp.aladin_compose.features.app.artifactpane.page

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.unit.dp
import com.jvp.aladin_compose.service.web.PageEditorBridge
import com.jvp.aladin_compose.service.web.WebWidget
import com.jvp.aladin_compose.ui_lib.AladinColor

@Composable
fun PageUi(
    state: PageEditorState,
    onDocumentUpdated: (pageId: String, markdown: String) -> Unit,
    onFileUploadRequested: (
        pageId: String,
        requestId: String,
        filename: String,
        contentType: String?,
        base64Data: String,
    ) -> Unit,
    pageEditorBridge: PageEditorBridge,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier =
            modifier.fillMaxSize()
                .background(AladinColor.Canvas)
                .padding(1.dp),
    ) {
        if (state.isLoaded) {
            Box(
                modifier =
                    Modifier.fillMaxSize()
                        .clipToBounds()
                        .background(AladinColor.Canvas),
            ) {
                WebWidget(
                    pageId = state.pageId,
                    initialMarkdown = state.initialMarkdown,
                    isEditable = state.isEditable,
                    onDocumentUpdated = onDocumentUpdated,
                    onFileUploadRequested = onFileUploadRequested,
                    pageEditorBridge = pageEditorBridge,
                    modifier = Modifier.fillMaxSize(),
                )
            }
        } else {
            Text(
                text = if (state.loadError == null) "Loading document..." else "Document failed to load.",
                color = AladinColor.InkMuted,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.align(Alignment.Center),
            )
        }
    }
}
