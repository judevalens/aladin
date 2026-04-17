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
        HStack(spacing: 0) {
            sidebarPane
                .frame(width: 320)

            DetailView(
                selectedSection: $sidebarVM.selectedSection,
                viewModel: detailVM
            )
            .frame(minWidth: 520, maxWidth: .infinity, maxHeight: .infinity)
        }
        .background(Color.appBackground)
        .preferredColorScheme(.dark)
        .task {
            await detailVM.refresh()
        }
    }

    private var sidebarPane: some View {
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
        .appGlassSurface(cornerRadius: 24, elevated: true)
        .padding(10)
        .background(Color.appBackground)
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
    }
}

extension Color {
    static let appBackground      = Color(red: 0.05, green: 0.05, blue: 0.055)  // #0D0D0E
    static let appChrome          = Color(red: 0.07, green: 0.07, blue: 0.08)   // #121214
    static let appSurface         = Color(red: 0.10, green: 0.10, blue: 0.115)  // #1A1A1D
    static let appElevatedSurface = Color(red: 0.14, green: 0.14, blue: 0.155)  // #242427
}

extension View {
    func appGlassSurface(cornerRadius: CGFloat, elevated: Bool = false) -> some View {
        modifier(AppGlassSurface(cornerRadius: cornerRadius, elevated: elevated))
    }

    func appGlassCapsule(elevated: Bool = false) -> some View {
        modifier(AppGlassCapsule(elevated: elevated))
    }
}

private struct AppGlassSurface: ViewModifier {
    let cornerRadius: CGFloat
    let elevated: Bool

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: cornerRadius, style: .continuous)
        content
            .background {
                shape
                    .fill(.ultraThinMaterial)
                    .overlay {
                        shape.fill((elevated ? Color.appElevatedSurface : Color.appSurface).opacity(elevated ? 0.58 : 0.46))
                    }
            }
            .overlay {
                shape.stroke(.white.opacity(elevated ? 0.22 : 0.16), lineWidth: 1)
            }
            .shadow(color: .black.opacity(elevated ? 0.22 : 0.14), radius: elevated ? 18 : 10, x: 0, y: elevated ? 10 : 4)
    }
}

private struct AppGlassCapsule: ViewModifier {
    let elevated: Bool

    func body(content: Content) -> some View {
        content
            .background {
                Capsule()
                    .fill(.ultraThinMaterial)
                    .overlay {
                        Capsule().fill((elevated ? Color.appElevatedSurface : Color.appSurface).opacity(elevated ? 0.58 : 0.46))
                    }
            }
            .overlay {
                Capsule().stroke(.white.opacity(elevated ? 0.20 : 0.14), lineWidth: 1)
            }
    }
}
