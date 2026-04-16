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
    @ObservedObject var model: AppModel
    @Binding var artifactTabs: [ArtifactTab]
    @Binding var activeArtifactTabID: ArtifactTab.ID

    var body: some View {
        Group {
            switch selectedSection {
            case .artifacts:
                ArtifactsPanel(
                    searchText: searchText,
                    model: model,
                    tabs: $artifactTabs,
                    activeTabID: $activeArtifactTabID
                )
            case .graph:
                PlaceholderPanel(
                    title: "Graph Canvas",
                    message: "Keep this surface native around the edges and embed the D3 scene inside a focused WKWebView when you are ready.",
                    cardTitle: "Next step",
                    cardBody: "Bridge the graph with a narrow message interface instead of turning the whole client into a web shell."
                )
            case .feed:
                PlaceholderPanel(
                    title: "Artifact Feed",
                    message: "This surface can become a native timeline over `/api/feed/` once you want the review stream online.",
                    cardTitle: "Current posture",
                    cardBody: "Keep feed separate from artifacts so the workspace shell stays legible."
                )
            case .insights:
                PlaceholderPanel(
                    title: "Insight Review",
                    message: "Use this area for accept and dismiss review flows over `/api/insights/` once you are ready to expose the moderation loop.",
                    cardTitle: "Current posture",
                    cardBody: "Keep the review surface narrow and decisive rather than folding it into the graph."
                )
            }
        }
    }
}

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
                .background(.background, in: RoundedRectangle(cornerRadius: 16, style: .continuous))

                MetricCard(title: cardTitle, value: title, note: cardBody)
            }
            .padding(20)
        }
    }
}
