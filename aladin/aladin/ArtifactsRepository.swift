//
//  ArtifactsRepository.swift
//  aladin
//

import Foundation

struct ArtifactSyncer: ArtifactSyncing {
    private let apiClient: APIClient

    init(apiClient: APIClient? = nil) {
        self.apiClient = apiClient ?? APIClient()
    }

    func fetchArtifacts() async throws -> [ArtifactSummary] {
        try await apiClient.fetchArtifacts()
    }
}

@MainActor
final class RemoteArtifactRepository: ArtifactRepository {
    private let syncer: any ArtifactSyncing
    private var latest: [ArtifactSummary] = []
    private var continuations: [UUID: AsyncStream<[ArtifactSummary]>.Continuation] = [:]

    init(syncer: (any ArtifactSyncing)? = nil) {
        self.syncer = syncer ?? ArtifactSyncer()
    }

    func artifacts() -> AsyncStream<[ArtifactSummary]> {
        let id = UUID()
        return AsyncStream { continuation in
            continuations[id] = continuation
            continuation.yield(latest)
            continuation.onTermination = { [weak self] _ in
                Task { @MainActor [weak self] in
                    self?.continuations.removeValue(forKey: id)
                }
            }
        }
    }

    func refresh() async throws {
        latest = try await syncer.fetchArtifacts()
        for continuation in continuations.values {
            continuation.yield(latest)
        }
    }
}
