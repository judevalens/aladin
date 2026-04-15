import SwiftUI

struct Sidebar: View {
    @Binding var selectedSection: HomeSection?
    let artifacts: [ArtifactSummary]
    let sources: [SourceSummary]
    let workerStatus: WorkerStatus?
    let isLoading: Bool
    let onNewNote: () -> Void

    var body: some View {
        VStack(spacing: 8) {
            SidebarHeader()

            List {
                Section("Workspace") {
                    ArtifactSidebarGroup(
                        selectedSection: $selectedSection,
                        artifacts: artifacts,
                        onNewNote: onNewNote
                    )

                    ForEach(HomeSection.allCases.filter { $0 != .artifacts }) { section in
                        Button {
                            selectedSection = section
                        } label: {
                            SidebarSectionRow(
                                section: section,
                                isSelected: selectedSection == section
                            )
                        }
                        .buttonStyle(.plain)
                    }
                }

                Section("Sources") {
                    if sources.isEmpty && isLoading {
                        Text("Loading sources...")
                            .foregroundStyle(.secondary)
                    } else if sources.isEmpty {
                        Text("No sources yet")
                            .foregroundStyle(.secondary)
                    } else {
                        ForEach(sources) { source in
                            SourceRow(source: source)
                        }
                    }
                }
            }
            .listStyle(.sidebar)
        }
        .safeAreaInset(edge: .bottom) {
            StatusFooter(workerStatus: workerStatus, sourceCount: sources.count)
                .padding(.horizontal, 12)
                .padding(.bottom, 12)
        }
    }
}

private struct ArtifactSidebarGroup: View {
    @Binding var selectedSection: HomeSection?
    let artifacts: [ArtifactSummary]
    let onNewNote: () -> Void
    @State private var isExpanded = true

    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            HStack(spacing: 6) {
                Button {
                    isExpanded.toggle()
                } label: {
                    Image(systemName: "chevron.right")
                        .font(.system(size: 11, weight: .semibold))
                        .rotationEffect(.degrees(isExpanded ? 90 : 0))
                        .frame(width: 18, height: 28)
                }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)

                Button {
                    selectedSection = .artifacts
                } label: {
                    SidebarSectionRow(
                        section: .artifacts,
                        isSelected: selectedSection == .artifacts
                    )
                }
                .buttonStyle(.plain)
            }

            if isExpanded {
                VStack(alignment: .leading, spacing: 2) {
                    Button {
                        selectedSection = .artifacts
                        onNewNote()
                    } label: {
                        ArtifactFolderRow(title: "New note", count: nil, systemImage: "plus")
                    }
                    .buttonStyle(.plain)

                    ArtifactFolderRow(title: "Inbox", count: artifacts.count, systemImage: "tray")
                    ArtifactFolderRow(title: "Writing", count: artifacts.filter(\.isWritingArtifact).count, systemImage: "square.and.pencil")
                    ArtifactFolderRow(title: "Links", count: artifacts.filter(\.isLinkArtifact).count, systemImage: "link")
                    ArtifactFolderRow(title: "Audio", count: artifacts.filter(\.isAudioArtifact).count, systemImage: "waveform")
                }
                .padding(.leading, 34)
                .padding(.top, 2)
            }
        }
    }
}

private struct SidebarSectionRow: View {
    let section: HomeSection
    let isSelected: Bool

    var body: some View {
        Label(section.title, systemImage: section.systemImage)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .background(
                isSelected ? Color.accentColor.opacity(0.16) : Color.clear,
                in: RoundedRectangle(cornerRadius: 8, style: .continuous)
            )
            .contentShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
    }
}

private struct ArtifactFolderRow: View {
    let title: String
    let count: Int?
    let systemImage: String

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: systemImage)
                .frame(width: 14)
                .foregroundStyle(.secondary)

            Text(title)
                .lineLimit(1)

            Spacer(minLength: 8)

            if let count {
                Text("\(count)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .font(.callout)
        .padding(.horizontal, 8)
        .padding(.vertical, 5)
        .contentShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
    }
}

private struct SidebarHeader: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 3) {
            Text("Aladin")
                .font(.title3.weight(.semibold))

            Text("Research Workspace")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.horizontal, 16)
        .padding(.top, 14)
        .padding(.bottom, 8)
    }
}

private extension ArtifactSummary {
    var isWritingArtifact: Bool {
        let normalized = type.lowercased()
        return normalized.contains("writing") || normalized.contains("note")
    }

    var isLinkArtifact: Bool {
        type.localizedCaseInsensitiveContains("link") || sourceURL != nil
    }

    var isAudioArtifact: Bool {
        let normalized = type.lowercased()
        return normalized.contains("audio") || normalized.contains("voice")
    }
}

private struct SourceRow: View {
    let source: SourceSummary

    var body: some View {
        HStack(spacing: 10) {
            Circle()
                .fill(source.syncState == "active" ? Color.green : Color.secondary.opacity(0.4))
                .frame(width: 8, height: 8)

            VStack(alignment: .leading, spacing: 2) {
                Text(source.name)
                    .lineLimit(1)

                Text(source.type.capitalized)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .padding(.vertical, 2)
    }
}

private struct StatusFooter: View {
    let workerStatus: WorkerStatus?
    let sourceCount: Int

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Label("Sync Status", systemImage: "waveform.path.ecg")
                .font(.footnote.weight(.semibold))

            Text("\(sourceCount) sources connected")
                .font(.caption)
                .foregroundStyle(.secondary)

            if let workerStatus {
                Text("\(workerStatus.queue.pending) active cycles, \(workerStatus.queue.pendingPipeline) pipeline backlog")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            } else {
                Text("Waiting for worker status")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(10)
        .background(.regularMaterial, in: RoundedRectangle(cornerRadius: 10, style: .continuous))
    }
}
