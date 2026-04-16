//
//  SidebarViewModel.swift
//  aladin
//

import Foundation
import Combine

@MainActor
final class SidebarViewModel: ObservableObject {
    @Published var selectedSection: HomeSection? = .artifacts

    let artifactService: ArtifactService
    let sourceService: SourceService
    let workerStatusService: WorkerStatusService

    var artifacts: [ArtifactSummary] { artifactService.artifacts }
    var sources: [SourceSummary] { sourceService.sources }
    var workerStatus: WorkerStatus? { workerStatusService.workerStatus }

    private var cancellables = Set<AnyCancellable>()

    init(
        artifactService: ArtifactService,
        sourceService: SourceService,
        workerStatusService: WorkerStatusService
    ) {
        self.artifactService = artifactService
        self.sourceService = sourceService
        self.workerStatusService = workerStatusService

        // Propagate service changes to trigger view updates
        artifactService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
        sourceService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
        workerStatusService.objectWillChange.sink { [weak self] in self?.objectWillChange.send() }.store(in: &cancellables)
    }
}
