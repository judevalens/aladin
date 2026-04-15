//
//  AppModel.swift
//  aladin
//

import Foundation
import Combine

@MainActor
final class AppModel: ObservableObject {
    @Published private(set) var artifacts: [ArtifactSummary] = []
    @Published private(set) var sources: [SourceSummary] = []
    @Published private(set) var workerStatus: WorkerStatus?
    @Published private(set) var isLoading = false
    @Published private(set) var lastUpdated: Date?
    @Published var errorMessage: String?

    private let artifactRepository: any ArtifactRepository
    private let sourceRepository: any SourceRepository
    private let workerStatusRepository: any WorkerStatusRepository
    private let captureRepository: any CaptureRepository

    private var artifactTask: Task<Void, Never>?
    private var sourceTask: Task<Void, Never>?
    private var workerStatusTask: Task<Void, Never>?

    init(
        artifactRepository: (any ArtifactRepository)? = nil,
        sourceRepository: (any SourceRepository)? = nil,
        workerStatusRepository: (any WorkerStatusRepository)? = nil,
        captureRepository: (any CaptureRepository)? = nil
    ) {
        self.artifactRepository = artifactRepository ?? RemoteArtifactRepository()
        self.sourceRepository = sourceRepository ?? RemoteSourceRepository()
        self.workerStatusRepository = workerStatusRepository ?? RemoteWorkerStatusRepository()
        self.captureRepository = captureRepository ?? RemoteCaptureRepository()
        bindStreams()
    }

    deinit {
        artifactTask?.cancel()
        sourceTask?.cancel()
        workerStatusTask?.cancel()
    }

    func refresh() async {
        isLoading = true
        errorMessage = nil

        do {
            async let artifactsRefresh: Void = artifactRepository.refresh()
            async let sourcesRefresh: Void = sourceRepository.refresh()
            async let workerRefresh: Void = workerStatusRepository.refresh()
            _ = try await (artifactsRefresh, sourcesRefresh, workerRefresh)
            lastUpdated = Date()
        } catch {
            errorMessage = error.localizedDescription
        }

        isLoading = false
    }

    func createLink(title: String, url: String, note: String) async -> Bool {
        await runCapture {
            try await captureRepository.createLink(title: title, url: url, note: note)
        }
    }

    func createWritingNode(title: String, content: String) async -> Bool {
        await runCapture {
            try await captureRepository.createWritingNode(title: title, content: content)
        }
    }

    func createVoiceNote(fileURL: URL, title: String, transcript: String) async -> Bool {
        await runCapture {
            try await captureRepository.createVoiceNote(fileURL: fileURL, title: title, transcript: transcript)
        }
    }

    private func bindStreams() {
        artifactTask = Task { [weak self] in
            guard let self else { return }
            for await items in artifactRepository.artifacts() {
                self.artifacts = items
            }
        }

        sourceTask = Task { [weak self] in
            guard let self else { return }
            for await items in sourceRepository.sources() {
                self.sources = items
            }
        }

        workerStatusTask = Task { [weak self] in
            guard let self else { return }
            for await status in workerStatusRepository.workerStatus() {
                self.workerStatus = status
            }
        }
    }

    private func runCapture(_ operation: () async throws -> Void) async -> Bool {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            try await operation()
            await refresh()
            return true
        } catch {
            errorMessage = error.localizedDescription
            return false
        }
    }
}
