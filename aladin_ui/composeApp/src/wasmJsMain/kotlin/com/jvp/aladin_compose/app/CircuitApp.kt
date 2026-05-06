package com.jvp.aladin_compose.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import com.jvp.aladin_compose.features.app.AppPresenter
import com.jvp.aladin_compose.features.app.AppScreen
import com.jvp.aladin_compose.features.app.AppUiFactory
import com.jvp.aladin_compose.features.app.artifactpane.WorkPaneProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.link.LinkStateProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.page.PageStateProducerImpl
import com.jvp.aladin_compose.features.app.artifactpane.voice.VoiceStateProducerImpl
import com.jvp.aladin_compose.features.app.browser.DocumentBrowserProducerImpl
import com.jvp.aladin_compose.features.app.sidebar.SidebarProducerImpl
import com.jvp.aladin_compose.repo.ApiFolderRepository
import com.jvp.aladin_compose.repo.ArtifactRepositoryImpl
import com.jvp.aladin_compose.repo.doa.InMemoryArtifactDoa
import com.jvp.aladin_compose.service.PageDocumentSyncerImpl
import com.jvp.aladin_compose.service.web.PageEditorBridgeImpl
import com.slack.circuit.foundation.Circuit
import com.slack.circuit.foundation.CircuitCompositionLocals
import com.slack.circuit.foundation.CircuitContent

@Composable
fun CircuitApp() {
    val scope = rememberCoroutineScope()
    val circuit = remember {
        val folderRepository = ApiFolderRepository()
        val artifactRepository = ArtifactRepositoryImpl(InMemoryArtifactDoa())
        val pageEditorBridge = PageEditorBridgeImpl()
        val pageDocumentSyncer = PageDocumentSyncerImpl(artifactRepository, scope)
        Circuit.Builder()
            .addPresenterFactory(
                AppPresenter.Factory(
                    sidebarProducer = SidebarProducerImpl(),
                    documentBrowserProducer =
                        DocumentBrowserProducerImpl(
                            folderRepository = folderRepository,
                            artifactRepository = artifactRepository,
                            scope = scope,
                        ),
                    artifactPaneProducer =
                        WorkPaneProducerImpl(
                            artifactRepository = artifactRepository,
                            pageStateProducer =
                                PageStateProducerImpl(
                                    artifactRepository = artifactRepository,
                                    pageDocumentSyncer = pageDocumentSyncer,
                                    scope = scope,
                                    pageEditorBridge = pageEditorBridge,
                                ),
                            linkStateProducer =
                                LinkStateProducerImpl(artifactRepository = artifactRepository),
                            voiceStateProducer =
                                VoiceStateProducerImpl(artifactRepository = artifactRepository),
                            pageEditorBridge = pageEditorBridge,
                        ),
                )
            )
            .addUiFactory(AppUiFactory())
            .build()
    }

    CircuitCompositionLocals(circuit) { CircuitContent(AppScreen) }
}
