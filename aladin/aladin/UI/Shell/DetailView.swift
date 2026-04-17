import SwiftUI

struct DetailView: View {
    @Binding var selectedSection: HomeSection?
    @ObservedObject var viewModel: DetailViewModel

    private var currentSection: HomeSection {
        selectedSection ?? .artifacts
    }

    var body: some View {
        MainPanel(
            selectedSection: currentSection,
            searchText: viewModel.searchText,
            viewModel: viewModel,
            artifactTabs: $viewModel.artifactTabs,
            activeArtifactTabID: $viewModel.activeArtifactTabID
        )
        .navigationTitle(currentSection.title)
        .sheet(item: $viewModel.activeCaptureSheet) { sheet in
            switch sheet {
            case .link:
                LinkCaptureSheet(viewModel: viewModel)
            case .writingNode:
                WritingNodeCaptureSheet(viewModel: viewModel)
            case .voiceNote:
                VoiceNoteCaptureSheet(viewModel: viewModel)
            }
        }
    }
}
