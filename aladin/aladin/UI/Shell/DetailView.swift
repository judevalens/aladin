import SwiftUI

struct DetailView: View {
    @Binding var selectedSection: HomeSection?
    @ObservedObject var viewModel: DetailViewModel

    private var currentSection: HomeSection {
        selectedSection ?? .artifacts
    }

    var body: some View {
        VStack(spacing: 0) {
            DetailHeader(
                selectedSection: currentSection,
                searchText: $viewModel.searchText,
                isLoading: viewModel.isLoading,
                onRefresh: {
                    Task {
                        await viewModel.refresh()
                    }
                },
                onCaptureSelection: { option in
                    switch option {
                    case .link:
                        viewModel.activeCaptureSheet = .link
                    case .voiceNote:
                        viewModel.activeCaptureSheet = .voiceNote
                    case .writingNode:
                        selectedSection = .artifacts
                        viewModel.openNewNoteTab()
                    }
                }
            )

            Divider()

            MainPanel(
                selectedSection: currentSection,
                searchText: viewModel.searchText,
                viewModel: viewModel,
                artifactTabs: $viewModel.artifactTabs,
                activeArtifactTabID: $viewModel.activeArtifactTabID
            )
        }
        .background(Color(nsColor: .controlBackgroundColor))
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

private struct DetailHeader: View {
    let selectedSection: HomeSection
    @Binding var searchText: String
    let isLoading: Bool
    let onRefresh: () -> Void
    let onCaptureSelection: (CaptureOption) -> Void

    var body: some View {
        HStack(alignment: .center, spacing: 16) {
            VStack(alignment: .leading, spacing: 3) {
                Text(selectedSection.title)
                    .font(.title2.weight(.semibold))

                Text(selectedSection.subtitle)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }

            Spacer(minLength: 12)

            TextField("Search sources, entities, artifacts", text: $searchText)
                .font(.body)
                .textFieldStyle(.roundedBorder)
                .frame(width: 280)
                .controlSize(.large)

            Button {
                onRefresh()
            } label: {
                Label("Refresh", systemImage: "arrow.clockwise")
            }
            .controlSize(.large)
            .disabled(isLoading)

            Menu {
                ForEach(CaptureOption.allCases) { option in
                    Button {
                        onCaptureSelection(option)
                    } label: {
                        Label(option.title, systemImage: option.systemImage)
                    }
                }
            } label: {
                Label("Capture", systemImage: "plus.circle")
            }
            .controlSize(.large)

            Button {
            } label: {
                Label("New Source", systemImage: "plus")
            }
            .controlSize(.large)
        }
        .padding(.horizontal, 20)
        .padding(.vertical, 16)
        .background(.bar)
    }
}
