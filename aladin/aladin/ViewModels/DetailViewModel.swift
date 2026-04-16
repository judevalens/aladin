//
//  DetailViewModel.swift
//  aladin
//

import Foundation
import Combine

@MainActor
final class DetailViewModel: ObservableObject {
    @Published var searchText = ""
    @Published var activeCaptureSheet: CaptureSheet?
    @Published var artifactTabs: [ArtifactTab] = [.home]
    @Published var activeArtifactTabID: ArtifactTab.ID = ArtifactTab.homeID
    @Published var isLoading = false
    @Published var errorMessage: String?
    @Published var lastUpdated: Date?

    let artifactService: ArtifactService
    let sourceService: SourceService
    let workerStatusService: WorkerStatusService
    let captureService: CaptureService

    private var cancellables = Set<AnyCancellable>()

    init(
        artifactService: ArtifactService,
        sourceService: SourceService,
        workerStatusService: WorkerStatusService,
        captureService: CaptureService
    ) {
        self.artifactService = artifactService
        self.sourceService = sourceService
        self.workerStatusService = workerStatusService
        self.captureService = captureService

        artifactService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
        sourceService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
        workerStatusService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
    }

    // MARK: - Data access

    var artifacts: [ArtifactSummary] { artifactService.artifacts }
    var sources: [SourceSummary] { sourceService.sources }
    var workerStatus: WorkerStatus? { workerStatusService.workerStatus }

    // MARK: - Actions

    func refresh() async {
        isLoading = true
        errorMessage = nil

        do {
            async let a: Void = artifactService.refresh()
            async let s: Void = sourceService.refresh()
            async let w: Void = workerStatusService.refresh()
            _ = try await (a, s, w)
            lastUpdated = Date()
        } catch {
            errorMessage = error.localizedDescription
        }

        isLoading = false
    }

    func createLink(title: String, url: String, note: String) async -> Bool {
        await runCapture {
            try await self.captureService.createLink(title: title, url: url, note: note)
        }
    }

    func createWritingNode(title: String, content: String) async -> Bool {
        await runCapture {
            try await self.captureService.createWritingNode(title: title, content: content)
        }
    }

    func createVoiceNote(fileURL: URL, title: String, transcript: String) async -> Bool {
        await runCapture {
            try await self.captureService.createVoiceNote(fileURL: fileURL, title: title, transcript: transcript)
        }
    }

    // MARK: - Navigation (replaces NotificationCenter)

    func openNewNoteTab() {
        let tab = ArtifactTab.noteDraft()
        artifactTabs.append(tab)
        activeArtifactTabID = tab.id
    }

    func openFilterTab(name: String) {
        let tab = ArtifactTab.filter(name: name)
        if let existing = artifactTabs.first(where: { $0.id == tab.id }) {
            activeArtifactTabID = existing.id
        } else {
            artifactTabs.append(tab)
            activeArtifactTabID = tab.id
        }
    }

    func openDocumentTab(artifactID: String) {
        guard let artifact = artifacts.first(where: { $0.id == artifactID }) else { return }
        let tab = ArtifactTab.document(artifact)
        if let existing = artifactTabs.first(where: { $0.id == tab.id }) {
            activeArtifactTabID = existing.id
        } else {
            artifactTabs.append(tab)
            activeArtifactTabID = tab.id
        }
    }

    // MARK: - Private

    private func runCapture(_ operation: () async throws -> Void) async -> Bool {
        isLoading = true
        errorMessage = nil
        defer { isLoading = false }

        do {
            try await operation()
            lastUpdated = Date()
            return true
        } catch {
            errorMessage = error.localizedDescription
            return false
        }
    }
}
