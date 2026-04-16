//
//  WorkerStatusService.swift
//  aladin
//

import Combine
import Foundation

@MainActor
final class WorkerStatusService: ObservableObject {
    @Published private(set) var workerStatus: WorkerStatus?

    private let repository: any WorkerStatusRepository
    private var streamTask: Task<Void, Never>?

    init(repository: any WorkerStatusRepository) {
        self.repository = repository
        bindStream()
    }

    deinit {
        streamTask?.cancel()
    }

    func refresh() async throws {
        try await repository.refresh()
    }

    private func bindStream() {
        streamTask = Task { [weak self] in
            guard let self else { return }
            for await status in repository.workerStatus() {
                self.workerStatus = status
            }
        }
    }
}
