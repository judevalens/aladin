//
//  SourceService.swift
//  aladin
//

import Combine
import Foundation

@MainActor
final class SourceService: ObservableObject {
    @Published private(set) var sources: [SourceSummary] = []

    private let repository: any SourceRepository
    private var streamTask: Task<Void, Never>?

    init(repository: any SourceRepository) {
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
            for await items in repository.sources() {
                self.sources = items
            }
        }
    }
}
