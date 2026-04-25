package com.jvp.aladin_compose.features.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.produceState
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.FolderWorkspace
import com.jvp.aladin_compose.service.FolderService

@Composable
fun folderChildrenProducer(
    service: FolderService,
    parentId: String?,
    refreshKey: Int,
): List<FolderNode> {
    return produceState(initialValue = emptyList(), service, parentId, refreshKey) {
        value = service.children(parentId)
    }.value
}

@Composable
fun folderBreadcrumbsProducer(
    service: FolderService,
    parentId: String?,
    refreshKey: Int,
): List<BreadcrumbItem> {
    return produceState(initialValue = emptyList(), service, parentId, refreshKey) {
        value = service.breadcrumbs(parentId)
    }.value
}

@Composable
fun folderWorkspaceProducer(
    service: FolderService,
    folderId: String?,
    refreshKey: Int,
): FolderWorkspace? {
    return produceState<FolderWorkspace?>(initialValue = null, service, folderId, refreshKey) {
        value = folderId?.let { service.workspace(it) }
    }.value
}
