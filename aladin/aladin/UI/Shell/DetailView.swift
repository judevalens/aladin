import SwiftUI

struct DetailView: View {
    @Binding var selectedSection: HomeSection?
    @Binding var searchText: String
    @ObservedObject var model: AppModel
    @State private var activeCaptureSheet: CaptureSheet?
    @State private var artifactTabs: [ArtifactTab] = [.home]
    @State private var activeArtifactTabID: ArtifactTab.ID = ArtifactTab.homeID

    private var currentSection: HomeSection {
        selectedSection ?? .artifacts
    }

    var body: some View {
        VStack(spacing: 0) {
            DetailHeader(
                selectedSection: currentSection,
                searchText: $searchText,
                isLoading: model.isLoading,
                onRefresh: {
                    Task {
                        await model.refresh()
                    }
                },
                onCaptureSelection: { option in
                    switch option {
                    case .link:
                        activeCaptureSheet = .link
                    case .voiceNote:
                        activeCaptureSheet = .voiceNote
                    case .writingNode:
                        openNewNoteTab()
                    }
                }
            )

            Divider()

            MainPanel(
                selectedSection: currentSection,
                searchText: searchText,
                model: model,
                artifactTabs: $artifactTabs,
                activeArtifactTabID: $activeArtifactTabID
            )
        }
        .background(Color(nsColor: .controlBackgroundColor))
        .onReceive(NotificationCenter.default.publisher(for: .aladinOpenNewNoteTab)) { _ in
            openNewNoteTab()
        }
        .sheet(item: $activeCaptureSheet) { sheet in
            switch sheet {
            case .link:
                LinkCaptureSheet(model: model)
            case .writingNode:
                WritingNodeCaptureSheet(model: model)
            case .voiceNote:
                VoiceNoteCaptureSheet(model: model)
            }
        }
    }

    private func openNewNoteTab() {
        selectedSection = .artifacts
        let tab = ArtifactTab.noteDraft()
        artifactTabs.append(tab)
        activeArtifactTabID = tab.id
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
