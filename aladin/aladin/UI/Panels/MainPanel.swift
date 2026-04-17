import SwiftUI

struct ArtifactTab: Identifiable, Equatable {
    enum Kind: Equatable {
        case home
        case noteDraft
        case artifactPreview
        case artifactFilter(String)
        case document(ArtifactSummary)
    }

    let id: String
    var kind: Kind
    var title = ""
    var noteContent = ""
    var artifact: ArtifactSummary?

    static let homeID = "artifact-home"

    static var home: ArtifactTab {
        ArtifactTab(id: homeID, kind: .home, title: "Home")
    }

    static func noteDraft() -> ArtifactTab {
        ArtifactTab(id: UUID().uuidString, kind: .noteDraft)
    }

    static func filter(name: String) -> ArtifactTab {
        ArtifactTab(id: "filter-\(name)", kind: .artifactFilter(name), title: name)
    }

    static func document(_ artifact: ArtifactSummary) -> ArtifactTab {
        // Stable ID so re-opening the same doc switches to the existing tab
        ArtifactTab(id: "doc-\(artifact.id)", kind: .document(artifact), title: artifact.label)
    }

    var displayTitle: String {
        switch kind {
        case .home:                     return "Home"
        case .artifactFilter(let name): return name
        case .document(let artifact):   return artifact.label
        default:
            if let artifact { return artifact.label }
            let trimmed = title.trimmingCharacters(in: .whitespacesAndNewlines)
            return trimmed.isEmpty ? "Untitled note" : trimmed
        }
    }

    var tabSystemImage: String {
        switch kind {
        case .home:             return "house"
        case .noteDraft:        return "square.and.pencil"
        case .artifactPreview:  return "doc"
        case .document:         return "doc.text"
        case .artifactFilter(let name):
            switch name {
            case "Inbox":   return "tray"
            case "Writing": return "square.and.pencil"
            case "Links":   return "link"
            case "Audio":   return "waveform"
            default:        return "folder"
            }
        }
    }

    var isClosable: Bool {
        kind != .home
    }
}

struct MainPanel: View {
    let selectedSection: HomeSection
    let searchText: String
    @ObservedObject var viewModel: DetailViewModel
    @Binding var artifactTabs: [ArtifactTab]
    @Binding var activeArtifactTabID: ArtifactTab.ID

    var body: some View {
        Group {
            switch selectedSection {
            case .artifacts:
                ArtifactsPanel(
                    searchText: searchText,
                    viewModel: viewModel,
                    tabs: $artifactTabs,
                    activeTabID: $activeArtifactTabID
                )
            case .graph:
                GraphPanel()
            case .feed:
                FeedPanel()
            case .insights:
                InsightsPanel()
            }
        }
    }
}

// MARK: – Graph

private struct GraphPanel: View {
    var body: some View {
        ZStack(alignment: .topLeading) {
            Color.appBackground
                .ignoresSafeArea()

            // Placeholder canvas
            Text("Canvas")
                .font(.system(size: 120, weight: .black))
                .foregroundStyle(.white.opacity(0.03))
                .frame(maxWidth: .infinity, maxHeight: .infinity)

            // Minimal top-left title
            VStack(alignment: .leading, spacing: 4) {
                Image(systemName: HomeSection.graph.systemImage)
                    .font(.system(size: 13))
                    .foregroundStyle(.secondary)
                Text(HomeSection.graph.title)
                    .font(.title3.weight(.semibold))
            }
            .padding(24)
        }
    }
}

// MARK: – Feed

private struct FeedPanel: View {
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack(alignment: .center) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(HomeSection.feed.title)
                        .font(.title3.weight(.semibold))
                    Text(HomeSection.feed.subtitle)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Spacer()

                Button {
                    // TODO: refresh
                } label: {
                    Label("Refresh", systemImage: "arrow.clockwise")
                        .font(.system(size: 12, weight: .medium))
                }
                .appGlassCapsule()
                .buttonStyle(.plain)
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 16)
            .background(Color.appBackground)

            Divider().opacity(0.15)

            ScrollView {
                VStack(spacing: 12) {
                    ForEach(0..<5) { _ in
                        RoundedRectangle(cornerRadius: 12, style: .continuous)
                            .fill(Color.appSurface)
                            .frame(maxWidth: .infinity)
                            .frame(height: 72)
                            .opacity(0.5)
                    }
                }
                .padding(20)
            }
            .background(Color.appBackground)
        }
    }
}

// MARK: – Insights

private struct InsightsPanel: View {
    var body: some View {
        VStack(spacing: 0) {
            // Header
            HStack(alignment: .center) {
                VStack(alignment: .leading, spacing: 2) {
                    Text(HomeSection.insights.title)
                        .font(.title3.weight(.semibold))
                    Text(HomeSection.insights.subtitle)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Spacer()

                // Pending badge
                HStack(spacing: 5) {
                    Circle()
                        .fill(Color.yellow.opacity(0.85))
                        .frame(width: 7, height: 7)
                    Text("0 pending")
                        .font(.caption.weight(.medium))
                        .foregroundStyle(.secondary)
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .appGlassCapsule()
            }
            .padding(.horizontal, 20)
            .padding(.vertical, 16)
            .background(Color.appBackground)

            Divider().opacity(0.15)

            ScrollView {
                VStack(spacing: 12) {
                    Text("No insights yet")
                        .font(.system(size: 14))
                        .foregroundStyle(.tertiary)
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                        .padding(.top, 60)
                }
                .padding(20)
            }
            .background(Color.appBackground)
        }
    }
}

// MARK: – Legacy placeholder

private struct PlaceholderPanel: View {
    let title: String
    let message: String
    let cardTitle: String
    let cardBody: String

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                VStack(alignment: .leading, spacing: 12) {
                    Text(title)
                        .font(.title2.weight(.semibold))
                    Text(message)
                        .foregroundStyle(.secondary)
                }
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(24)
                .appGlassSurface(cornerRadius: 16)

                MetricCard(title: cardTitle, value: title, note: cardBody)
            }
            .padding(20)
        }
        .background(Color.appBackground)
    }
}
