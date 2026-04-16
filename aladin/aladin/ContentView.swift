//
//  ContentView.swift
//  aladin
//
//  Created by Jude Paulemon on 4/8/26.
//

import SwiftUI

struct ContentView: View {
    @StateObject private var sidebarVM: SidebarViewModel
    @StateObject private var detailVM: DetailViewModel

    init() {
        let artifactRepo = RemoteArtifactRepository()
        let sourceRepo = RemoteSourceRepository()
        let workerStatusRepo = RemoteWorkerStatusRepository()
        let captureRepo = RemoteCaptureRepository()

        let artifactService = ArtifactService(repository: artifactRepo)
        let sourceService = SourceService(repository: sourceRepo)
        let workerStatusService = WorkerStatusService(repository: workerStatusRepo)
        let captureService = CaptureService(repository: captureRepo, artifactService: artifactService)

        _sidebarVM = StateObject(wrappedValue: SidebarViewModel(
            artifactService: artifactService,
            sourceService: sourceService,
            workerStatusService: workerStatusService
        ))

        _detailVM = StateObject(wrappedValue: DetailViewModel(
            artifactService: artifactService,
            sourceService: sourceService,
            workerStatusService: workerStatusService,
            captureService: captureService
        ))
    }

    var body: some View {
        NavigationSplitView {
            Sidebar(
                selectedSection: $sidebarVM.selectedSection,
                artifacts: sidebarVM.artifacts,
                sources: sidebarVM.sources,
                workerStatus: sidebarVM.workerStatus,
                isLoading: detailVM.isLoading,
                onNewNote: {
                    sidebarVM.selectedSection = .artifacts
                    detailVM.openNewNoteTab()
                },
                onSelectFilter: { filterName in
                    sidebarVM.selectedSection = .artifacts
                    detailVM.openFilterTab(name: filterName)
                },
                onOpenDocument: { artifact in
                    sidebarVM.selectedSection = .artifacts
                    detailVM.openDocumentTab(artifactID: artifact.id)
                }
            )
            .navigationSplitViewColumnWidth(min: 250, ideal: 320, max: 380)
        } detail: {
            DetailView(
                selectedSection: $sidebarVM.selectedSection,
                viewModel: detailVM
            )
        }
        .navigationSplitViewStyle(.balanced)
        .task {
            await detailVM.refresh()
        }
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
    }
}
