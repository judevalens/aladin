//
//  SourcesRepository.swift
//  aladin
//

import Foundation

struct SourceSyncer: SourceSyncing {
    private let apiClient: APIClient

    init(apiClient: APIClient? = nil) {
        self.apiClient = apiClient ?? APIClient()
    }

    func fetchSources() async throws -> [SourceSummary] {
        try await apiClient.fetchSources()
    }
}

@MainActor
final class RemoteSourceRepository: SourceRepository {
    private let syncer: any SourceSyncing
    private var latest: [SourceSummary] = []
    private var continuations: [UUID: AsyncStream<[SourceSummary]>.Continuation] = [:]

    init(syncer: (any SourceSyncing)? = nil) {
        self.syncer = syncer ?? SourceSyncer()
    }

    func sources() -> AsyncStream<[SourceSummary]> {
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
        latest = try await syncer.fetchSources()
        for continuation in continuations.values {
            continuation.yield(latest)
        }
    }
}
