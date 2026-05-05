package com.jvp.aladin_compose.repo.doa

import com.jvp.aladin_compose.api.ApiClient
import com.jvp.aladin_compose.api.PageDocumentRecord
import com.jvp.aladin_compose.api.PageSaveRequest
import com.jvp.aladin_compose.api.UserArtifact
import com.jvp.aladin_compose.api.UserArtifactUpdateRequest

interface ArtifactDoa {
    suspend fun getArtifact(id: String): UserArtifact?

    suspend fun savePage(id: String, markdown: String, revision: Long): PageDocumentRecord

    suspend fun getPage(id: String): PageDocumentRecord?

    suspend fun renameArtifact(id: String, title: String): UserArtifact
}

class InMemoryArtifactDoa() : ArtifactDoa {
    override suspend fun getArtifact(id: String): UserArtifact {
        return ApiClient.getUserArtifact(id)
    }

    override suspend fun savePage(id: String, markdown: String, revision: Long): PageDocumentRecord {
        return ApiClient.savePage(id, PageSaveRequest(content = markdown, revision = revision))
    }

    override suspend fun getPage(id: String): PageDocumentRecord {
        return ApiClient.getPage(id)
    }

    override suspend fun renameArtifact(id: String, title: String): UserArtifact {
        return ApiClient.updateUserArtifact(id, UserArtifactUpdateRequest(title = title))
    }
}
