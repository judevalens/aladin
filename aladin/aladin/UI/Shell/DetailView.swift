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
        .background(Color(nsColor: .controlBackgroundColor))
        .navigationTitle(currentSection.title)
        .navigationSubtitle(currentSection.subtitle)
        .toolbar {
            ToolbarItem(placement: .automatic) {
                TextField("Search sources, entities, artifacts", text: $viewModel.searchText)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 240)
            }

            ToolbarItem(placement: .automatic) {
                Button {
                    Task { await viewModel.refresh() }
                } label: {
                    Label("Refresh", systemImage: "arrow.clockwise")
                }
                .disabled(viewModel.isLoading)
            }

            ToolbarItem(placement: .automatic) {
                Menu {
                    ForEach(CaptureOption.allCases) { option in
                        Button {
                            switch option {
                            case .link:
                                viewModel.activeCaptureSheet = .link
                            case .voiceNote:
                                viewModel.activeCaptureSheet = .voiceNote
                            case .writingNode:
                                selectedSection = .artifacts
                                viewModel.openNewNoteTab()
                            }
                        } label: {
                            Label(option.title, systemImage: option.systemImage)
                        }
                    }
                } label: {
                    Label("Capture", systemImage: "plus.circle")
                }
            }

            ToolbarItem(placement: .automatic) {
                Button {
                } label: {
                    Label("New Source", systemImage: "plus")
                }
            }
        }
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
