package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.ArtifactPreview
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.BrowserNodeKind
import com.jvp.aladin_compose.model.BrowserTreeNode
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.repo.FolderRepository

private const val RootLabel = "Folders"

class FolderService(
    private val repository: FolderRepository,
) {
    private var treeLoaded = false
    private var treeRoots: List<BrowserTreeNode> = emptyList()
    private var nodeById: Map<String, BrowserTreeNode> = emptyMap()
    private var childrenByParentId: Map<String?, List<BrowserTreeNode>> = emptyMap()
    private var parentById: Map<String, String?> = emptyMap()
    private var folderById: Map<String, FolderNode> = emptyMap()
    private var artifactPreviewById: Map<String, ArtifactPreview> = emptyMap()

    private val artifactById = mutableMapOf<String, Artifact>()

    suspend fun prepareBrowser() {
        ensureTreeLoaded()
    }

    suspend fun refreshBrowser() {
        treeLoaded = false
        treeRoots = emptyList()
        nodeById = emptyMap()
        childrenByParentId = emptyMap()
        parentById = emptyMap()
        folderById = emptyMap()
        artifactPreviewById = emptyMap()
        artifactById.clear()
        ensureTreeLoaded()
    }

    fun treeChildren(parentId: String?): List<BrowserTreeNode> = childrenByParentId[parentId].orEmpty()

    suspend fun folder(folderId: String?): FolderNode? {
        ensureTreeLoaded()
        return folderId?.let { folderById[it] }
    }

    fun artifactPreview(artifactId: String?): ArtifactPreview? = artifactId?.let { artifactPreviewById[it] }

    suspend fun artifact(artifactId: String?): Artifact? {
        val id = artifactId ?: return null
        artifactById[id]?.let { return it }
        return repository.artifact(id).also { artifactById[id] = it }
    }

    suspend fun folderBreadcrumbs(folderId: String?): List<BreadcrumbItem> {
        ensureTreeLoaded()
        return buildFolderBreadcrumbs(folderId)
    }

    suspend fun artifactBreadcrumbs(artifactId: String?): List<BreadcrumbItem> {
        ensureTreeLoaded()
        val id = artifactId ?: return listOf(BreadcrumbItem(id = null, label = RootLabel))
        val preview = artifactPreviewById[id]
        return buildFolderBreadcrumbs(parentById[id]) + BreadcrumbItem(id = id, label = preview?.title ?: "Artifact")
    }

    suspend fun ancestorFolderIds(folderId: String?): Set<String> {
        ensureTreeLoaded()
        val ids = linkedSetOf<String>()
        var currentId = folderId
        while (currentId != null) {
            val current = nodeById[currentId] ?: break
            if (current.kind == BrowserNodeKind.Folder) {
                ids += currentId
            }
            currentId = parentById[currentId]
        }
        return ids
    }

    suspend fun ancestorFolderIdsForArtifact(artifactId: String?): Set<String> {
        ensureTreeLoaded()
        val id = artifactId ?: return emptySet()
        return ancestorFolderIds(parentById[id])
    }

    suspend fun createFolder(parentId: String?): FolderNode {
        ensureTreeLoaded()
        val siblingCount = treeChildren(parentId).count { it.kind == BrowserNodeKind.Folder } + 1
        return repository.createFolder(parentId, "New Folder $siblingCount")
    }

    suspend fun createArtifact(folderId: String?, kind: ArtifactKind = ArtifactKind.Note): Artifact {
        ensureTreeLoaded()
        val siblingCount = treeChildren(folderId).count { it.kind == BrowserNodeKind.Artifact } + 1
        return repository.createArtifact(folderId, "New Artifact $siblingCount", kind)
    }

    fun rootNodes(): List<BrowserTreeNode> = treeRoots

    private suspend fun ensureTreeLoaded() {
        if (treeLoaded) return

        val roots = repository.browserTree()
        val nextNodeById = linkedMapOf<String, BrowserTreeNode>()
        val nextChildrenByParentId = linkedMapOf<String?, MutableList<BrowserTreeNode>>()
        val nextParentById = linkedMapOf<String, String?>()
        val nextFolderById = linkedMapOf<String, FolderNode>()
        val nextArtifactPreviewById = linkedMapOf<String, ArtifactPreview>()

        fun index(nodes: List<BrowserTreeNode>, parentId: String?) {
            val siblings = nextChildrenByParentId.getOrPut(parentId) { mutableListOf() }
            nodes.forEach { node ->
                siblings += node
                nextNodeById[node.id] = node
                nextParentById[node.id] = node.parentId
                when (node.kind) {
                    BrowserNodeKind.Folder -> {
                        nextFolderById[node.id] = FolderNode(node.id, node.parentId, node.title)
                    }
                    BrowserNodeKind.Artifact -> {
                        node.artifactPreview?.let { nextArtifactPreviewById[it.id] = it }
                    }
                }
                index(node.children, node.id)
            }
        }

        index(roots, parentId = null)

        treeRoots = roots
        nodeById = nextNodeById
        childrenByParentId = nextChildrenByParentId.mapValues { it.value.toList() }
        parentById = nextParentById
        folderById = nextFolderById
        artifactPreviewById = nextArtifactPreviewById
        treeLoaded = true
    }

    private fun buildFolderBreadcrumbs(folderId: String?): List<BreadcrumbItem> {
        if (folderId == null) return listOf(BreadcrumbItem(id = null, label = RootLabel))

        val crumbs = mutableListOf<FolderNode>()
        var currentId: String? = folderId
        while (currentId != null) {
            val current = folderById[currentId] ?: break
            crumbs += current
            currentId = parentById[current.id]
        }

        return buildList {
            add(BreadcrumbItem(id = null, label = RootLabel))
            crumbs.asReversed().forEach { add(BreadcrumbItem(id = it.id, label = it.title)) }
        }
    }
}
