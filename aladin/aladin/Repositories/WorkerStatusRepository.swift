//
//  WorkerStatusRepository.swift
//  aladin
//

import Foundation

struct WorkerStatusSyncer: WorkerStatusSyncing {
    private let apiClient: APIClient

    init(apiClient: APIClient? = nil) {
        self.apiClient = apiClient ?? APIClient()
    }

    func fetchWorkerStatus() async throws -> WorkerStatus {
        try await apiClient.fetchWorkerStatus()
    }
}

@MainActor
final class RemoteWorkerStatusRepository: WorkerStatusRepository {
    private let syncer: any WorkerStatusSyncing
    private var latest: WorkerStatus?
    private var continuations: [UUID: AsyncStream<WorkerStatus>.Continuation] = [:]

    init(syncer: (any WorkerStatusSyncing)? = nil) {
        self.syncer = syncer ?? WorkerStatusSyncer()
    }

    func workerStatus() -> AsyncStream<WorkerStatus> {
        let id = UUID()
        return AsyncStream { continuation in
            continuations[id] = continuation
            if let latest {
                continuation.yield(latest)
            }
            continuation.onTermination = { [weak self] _ in
                Task { @MainActor [weak self] in
                    self?.continuations.removeValue(forKey: id)
                }
            }
        }
    }

    func refresh() async throws {
        latest = try await syncer.fetchWorkerStatus()
        if let latest {
            for continuation in continuations.values {
                continuation.yield(latest)
            }
        }
    }
}
