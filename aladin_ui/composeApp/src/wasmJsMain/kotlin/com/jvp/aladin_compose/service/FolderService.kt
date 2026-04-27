package com.jvp.aladin_compose.service

import com.jvp.aladin_compose.model.Artifact
import com.jvp.aladin_compose.model.ArtifactKind
import com.jvp.aladin_compose.model.BreadcrumbItem
import com.jvp.aladin_compose.model.FolderNode
import com.jvp.aladin_compose.model.FolderTreeNode
import com.jvp.aladin_compose.repo.FolderRepository

private const val RootLabel = "Folders"

class FolderService(
    private val repository: FolderRepository,
) {
    private var treeLoaded = false
    private var folderTree: List<FolderTreeNode> = emptyList()
    private var folderById: Map<String, FolderNode> = emptyMap()
    private var childrenByParentId: Map<String?, List<FolderNode>> = emptyMap()
    private var parentById: Map<String, String?> = emptyMap()

    private val artifactsByFolderId = mutableMapOf<String?, List<Artifact>>()
    private val artifactById = mutableMapOf<String, Artifact>()

    suspend fun prepareBrowser(
        scopeId: String?,
        expandedFolderIds: Set<String>,
        drillDepth: Int,
    ) {
        ensureTreeLoaded()
        ensureArtifactsLoaded(scopeId)
        prepareExpandedScopes(scopeId, expandedFolderIds, drillDepth)
    }

    suspend fun refreshBrowser(scopeId: String?) {
        treeLoaded = false
        folderTree = emptyList()
        folderById = emptyMap()
        childrenByParentId = emptyMap()
        parentById = emptyMap()
        artifactsByFolderId.clear()
        artifactById.clear()
        prepareBrowser(scopeId = scopeId, expandedFolderIds = emptySet(), drillDepth = 0)
    }

    suspend fun folders(parentId: String?): List<FolderNode> {
        ensureTreeLoaded()
        return cachedFolders(parentId)
    }

    fun cachedFolders(parentId: String?): List<FolderNode> = childrenByParentId[parentId].orEmpty()

    suspend fun artifacts(folderId: String?): List<Artifact> {
        ensureArtifactsLoaded(folderId)
        return cachedArtifacts(folderId)
    }

    fun cachedArtifacts(folderId: String?): List<Artifact> = artifactsByFolderId[folderId].orEmpty()

    suspend fun folder(folderId: String?): FolderNode? {
        ensureTreeLoaded()
        return folderId?.let { folderById[it] }
    }

    suspend fun artifact(artifactId: String?): Artifact? {
        val id = artifactId ?: return null
        artifactById[id]?.let { return it }
        return repository.artifact(id).also { artifactById[id] = it }
    }

    suspend fun folderBreadcrumbs(folderId: String?): List<BreadcrumbItem> {
        ensureTreeLoaded()
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

    suspend fun artifactBreadcrumbs(artifactId: String?): List<BreadcrumbItem> {
        val artifact = artifact(artifactId) ?: return listOf(BreadcrumbItem(id = null, label = RootLabel))
        return folderBreadcrumbs(artifact.folderId) + BreadcrumbItem(id = artifact.id, label = artifact.title)
    }

    suspend fun ancestorFolderIds(folderId: String?): Set<String> {
        ensureTreeLoaded()
        val ids = linkedSetOf<String>()
        var currentId = folderId
        while (currentId != null) {
            ids += currentId
            currentId = parentById[currentId]
        }
        return ids
    }

    suspend fun ancestorFolderIdsForArtifact(artifactId: String?): Set<String> {
        val artifact = artifact(artifactId) ?: return emptySet()
        return ancestorFolderIds(artifact.folderId)
    }

    suspend fun createFolder(parentId: String?): FolderNode {
        val siblingCount = folders(parentId).size + 1
        return repository.createFolder(parentId, "New Folder $siblingCount")
    }

    suspend fun createArtifact(folderId: String?, kind: ArtifactKind = ArtifactKind.Note): Artifact {
        val siblingCount = artifacts(folderId).size + 1
        return repository.createArtifact(folderId, "New Artifact $siblingCount", kind)
    }

    fun hasCachedArtifacts(folderId: String?): Boolean = artifactsByFolderId.containsKey(folderId)

    fun treeRoots(): List<FolderNode> = cachedFolders(parentId = null)

    private suspend fun ensureTreeLoaded() {
        if (treeLoaded) return

        val tree = repository.folderTree()
        val nextFolderById = linkedMapOf<String, FolderNode>()
        val nextChildrenByParentId = linkedMapOf<String?, MutableList<FolderNode>>()
        val nextParentById = linkedMapOf<String, String?>()

        fun index(nodes: List<FolderTreeNode>, parentId: String?) {
            val siblings = nextChildrenByParentId.getOrPut(parentId) { mutableListOf() }
            nodes.forEach { node ->
                val folder = FolderNode(id = node.id, parentId = node.parentId, title = node.title)
                siblings += folder
                nextFolderById[folder.id] = folder
                nextParentById[folder.id] = folder.parentId
                index(node.children, folder.id)
            }
        }

        index(tree, parentId = null)

        folderTree = tree
        folderById = nextFolderById
        childrenByParentId = nextChildrenByParentId.mapValues { it.value.toList() }
        parentById = nextParentById
        treeLoaded = true
    }

    private suspend fun ensureArtifactsLoaded(folderId: String?) {
        if (artifactsByFolderId.containsKey(folderId)) return
        val artifacts = repository.artifacts(folderId)
        artifactsByFolderId[folderId] = artifacts
        artifacts.forEach { artifactById[it.id] = it }
    }

    private suspend fun prepareExpandedScopes(
        scopeId: String?,
        expandedFolderIds: Set<String>,
        drillDepth: Int,
    ) {
        suspend fun visit(parentId: String?, remainingDepth: Int) {
            val folders = cachedFolders(parentId)
            folders.forEach { folder ->
                if (folder.id !in expandedFolderIds) return@forEach
                ensureArtifactsLoaded(folder.id)
                if (remainingDepth > 0) {
                    visit(folder.id, remainingDepth - 1)
                }
            }
        }

        visit(scopeId, drillDepth)
    }
}
