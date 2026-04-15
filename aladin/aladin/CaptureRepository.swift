//
//  CaptureRepository.swift
//  aladin
//

import Foundation

protocol CaptureRepository {
    func createLink(title: String, url: String, note: String) async throws
    func createWritingNode(title: String, content: String) async throws
    func createVoiceNote(fileURL: URL, title: String, transcript: String) async throws
}

struct RemoteCaptureRepository: CaptureRepository {
    private let apiClient: APIClient

    init(apiClient: APIClient? = nil) {
        self.apiClient = apiClient ?? APIClient()
    }

    func createLink(title: String, url: String, note: String) async throws {
        let trimmedURL = url.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedNote = note.trimmingCharacters(in: .whitespacesAndNewlines)
        let label = trimmedTitle.isEmpty ? trimmedURL : trimmedTitle
        let content = trimmedNote.isEmpty ? trimmedURL : trimmedNote
        try await apiClient.createArtifact(type: "link", label: label, content: content, sourceURL: trimmedURL)
    }

    func createWritingNode(title: String, content: String) async throws {
        try await apiClient.createNote(label: title, content: content)
    }

    func createVoiceNote(fileURL: URL, title: String, transcript: String) async throws {
        let upload = try await apiClient.uploadAudio(fileURL: fileURL)
        let trimmedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
        let fallbackTitle = fileURL.deletingPathExtension().lastPathComponent
        let label = trimmedTitle.isEmpty ? fallbackTitle : trimmedTitle
        let body = transcript.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            ? "Voice note uploaded from \(fileURL.lastPathComponent)"
            : transcript.trimmingCharacters(in: .whitespacesAndNewlines)

        try await apiClient.createArtifact(
            type: "voice_note",
            label: label,
            content: body,
            sourceURL: upload.url
        )
    }
}
