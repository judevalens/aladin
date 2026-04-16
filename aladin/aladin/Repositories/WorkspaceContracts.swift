//
//  WorkspaceContracts.swift
//  aladin
//

import Foundation

protocol ArtifactRepository: AnyObject {
    func artifacts() -> AsyncStream<[ArtifactSummary]>
    func refresh() async throws
}

protocol SourceRepository: AnyObject {
    func sources() -> AsyncStream<[SourceSummary]>
    func refresh() async throws
}

protocol WorkerStatusRepository: AnyObject {
    func workerStatus() -> AsyncStream<WorkerStatus>
    func refresh() async throws
}

protocol ArtifactSyncing {
    func fetchArtifacts() async throws -> [ArtifactSummary]
}

protocol SourceSyncing {
    func fetchSources() async throws -> [SourceSummary]
}

protocol WorkerStatusSyncing {
    func fetchWorkerStatus() async throws -> WorkerStatus
}
