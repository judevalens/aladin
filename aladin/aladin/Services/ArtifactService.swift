//
//  ArtifactService.swift
//  aladin
//

import Combine
import Foundation

@MainActor
final class ArtifactService: ObservableObject {
    @Published private(set) var artifacts: [ArtifactSummary] = []

    private let repository: any ArtifactRepository
    private var streamTask: Task<Void, Never>?

    init(repository: any ArtifactRepository) {
        self.repository = repository
        bindStream()
    }

    deinit {
        streamTask?.cancel()
    }

    func refresh() async throws {
        try await repository.refresh()
    }

    func filtered(by query: String) -> [ArtifactSummary] {
        let needle = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !needle.isEmpty else { return artifacts }
        return artifacts.filter { artifact in
            artifact.label.localizedCaseInsensitiveContains(needle)
            || artifact.type.localizedCaseInsensitiveContains(needle)
            || artifact.content.localizedCaseInsensitiveContains(needle)
        }
    }

    private func bindStream() {
        streamTask = Task { [weak self] in
            guard let self else { return }
            for await items in repository.artifacts() {
                self.artifacts = items
            }
        }
    }
}
