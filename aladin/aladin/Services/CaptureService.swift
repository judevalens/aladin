//
//  CaptureService.swift
//  aladin
//

import Foundation

@MainActor
final class CaptureService {
    private let repository: any CaptureRepository
    private let artifactService: ArtifactService

    init(repository: any CaptureRepository, artifactService: ArtifactService) {
        self.repository = repository
        self.artifactService = artifactService
    }

    func createLink(title: String, url: String, note: String) async throws {
        try await repository.createLink(title: title, url: url, note: note)
        try await artifactService.refresh()
    }

    func createWritingNode(title: String, content: String) async throws {
        try await repository.createWritingNode(title: title, content: content)
        try await artifactService.refresh()
    }

    func createVoiceNote(fileURL: URL, title: String, transcript: String) async throws {
        try await repository.createVoiceNote(fileURL: fileURL, title: title, transcript: transcript)
        try await artifactService.refresh()
    }
}
