import SwiftUI

struct Sidebar: View {
    @Binding var selectedSection: HomeSection?
    let artifacts: [ArtifactSummary]
    let sources: [SourceSummary]
    let workerStatus: WorkerStatus?
    let isLoading: Bool
    let onNewNote: () -> Void
    let onSelectFilter: (String) -> Void
    let onOpenDocument: (ArtifactSummary) -> Void

    var body: some View {
        VStack(spacing: 0) {
            SidebarHeader()

            Divider()
                .opacity(0.5)

            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    SidebarSectionLabel("Workspace")

                    ArtifactSidebarGroup(
                        selectedSection: $selectedSection,
                        artifacts: artifacts,
                        onNewNote: onNewNote,
                        onSelectFilter: onSelectFilter,
                        onOpenDocument: onOpenDocument
                    )

                    ForEach(HomeSection.allCases.filter { $0 != .artifacts }) { section in
                        SidebarNavRow(
                            section: section,
                            isSelected: selectedSection == section
                        ) {
                            selectedSection = section
                        }
                    }

                    SidebarSectionLabel("Sources")

                    if sources.isEmpty && isLoading {
                        SidebarEmptyRow(text: "Loading sources…")
                    } else if sources.isEmpty {
                        SidebarEmptyRow(text: "No sources yet")
                    } else {
                        ForEach(sources) { source in
                            SourceRow(source: source)
                        }
                    }
                }
                .padding(.horizontal, 8)
                .padding(.bottom, 12)
            }
        }
        .safeAreaInset(edge: .bottom) {
            StatusFooter(workerStatus: workerStatus, sourceCount: sources.count)
                .padding(.horizontal, 12)
                .padding(.bottom, 12)
        }
    }
}

// MARK: – Section label

private struct SidebarSectionLabel: View {
    let title: String
    init(_ title: String) { self.title = title }

    var body: some View {
        Text(title)
            .font(.system(size: 12, weight: .medium))
            .foregroundStyle(.secondary)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 12)
            .padding(.top, 18)
            .padding(.bottom, 4)
    }
}

// MARK: – Nav row

private struct SidebarNavRow: View {
    let section: HomeSection
    let isSelected: Bool
    let action: () -> Void
    @State private var isHovering = false

    var body: some View {
        Button(action: action) {
            HStack(spacing: 10) {
                Image(systemName: section.systemImage)
                    .font(.system(size: 14, weight: .regular))
                    .foregroundStyle(isSelected ? Color.accentColor : .secondary)
                    .frame(width: 20, alignment: .center)

                Text(section.title)
                    .font(.system(size: 14))
                    .foregroundStyle(isSelected ? Color.accentColor : .primary)

                Spacer()
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 7)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                isSelected
                    ? Color.accentColor.opacity(0.12)
                    : (isHovering ? Color.primary.opacity(0.05) : Color.clear),
                in: RoundedRectangle(cornerRadius: 8, style: .continuous)
            )
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .onHover { isHovering = $0 }
    }
}

// MARK: – Artifact group

private struct ArtifactSidebarGroup: View {
    @Binding var selectedSection: HomeSection?
    let artifacts: [ArtifactSummary]
    let onNewNote: () -> Void
    let onSelectFilter: (String) -> Void
    let onOpenDocument: (ArtifactSummary) -> Void
    @State private var isExpanded = true
    @State private var isHovering = false
    @State private var activeFilter: String? = nil

    private var writingArtifacts: [ArtifactSummary] {
        Array(artifacts.filter(\.isWritingArtifact).prefix(3))
    }

    private var isSelected: Bool { selectedSection == .artifacts }

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Header row
            Button {
                selectedSection = .artifacts
                withAnimation(.easeOut(duration: 0.2)) { isExpanded.toggle() }
            } label: {
                HStack(spacing: 10) {
                    Image(systemName: HomeSection.artifacts.systemImage)
                        .font(.system(size: 14))
                        .foregroundStyle(isSelected ? Color.accentColor : .secondary)
                        .frame(width: 20, alignment: .center)

                    Text(HomeSection.artifacts.title)
                        .font(.system(size: 14))
                        .foregroundStyle(isSelected ? Color.accentColor : .primary)

                    Spacer()

                    Image(systemName: "chevron.down")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(.secondary)
                        .rotationEffect(.degrees(isExpanded ? 180 : 0))
                }
                .padding(.horizontal, 10)
                .padding(.vertical, 9)
                .frame(maxWidth: .infinity, alignment: .leading)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .onHover { isHovering = $0 }

            // Sub-items — full width, no indentation
            if isExpanded {
                VStack(alignment: .leading, spacing: 2) {
                    Button {
                        selectedSection = .artifacts
                        onNewNote()
                    } label: {
                        ArtifactSubRow(title: "New note", systemImage: "plus", count: nil, isAction: true)
                    }
                    .buttonStyle(.plain)

                    filterButton("Inbox",  systemImage: "tray",     count: artifacts.count)
                    WritingSubGroup(
                        recentDocs: writingArtifacts,
                        totalCount: artifacts.filter(\.isWritingArtifact).count,
                        isActive: activeFilter == "Writing",
                        onSelect: {
                            activeFilter = "Writing"
                            onSelectFilter("Writing")
                        },
                        onOpenDocument: onOpenDocument
                    )
                    filterButton("Links",  systemImage: "link",     count: artifacts.filter(\.isLinkArtifact).count)
                    filterButton("Audio",  systemImage: "waveform", count: artifacts.filter(\.isAudioArtifact).count)
                }
                .padding(.bottom, 8)
            }
        }
        .background(isExpanded ? Color.primary.opacity(0.05) : Color.clear)
        .clipShape(RoundedRectangle(cornerRadius: 10, style: .continuous))
        .padding(.bottom, isExpanded ? 6 : 0)
    }

    @ViewBuilder
    private func filterButton(_ name: String, systemImage: String, count: Int) -> some View {
        Button {
            activeFilter = name
            onSelectFilter(name)
        } label: {
            ArtifactSubRow(
                title: name,
                systemImage: systemImage,
                count: count,
                isSelected: activeFilter == name
            )
        }
        .buttonStyle(.plain)
    }
}

// MARK: – Writing sub-group (selectable row + expandable recent docs)

private struct WritingSubGroup: View {
    let recentDocs: [ArtifactSummary]
    let totalCount: Int
    let isActive: Bool
    let onSelect: () -> Void
    let onOpenDocument: (ArtifactSummary) -> Void

    @State private var isExpanded = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            // Writing row — acts like a filter button but also has a chevron expand
            Button {
                onSelect()
                if !recentDocs.isEmpty {
                    withAnimation(.easeOut(duration: 0.18)) { isExpanded.toggle() }
                }
            } label: {
                ArtifactSubRow(
                    title: "Writing",
                    systemImage: "square.and.pencil",
                    count: totalCount,
                    isSelected: isActive,
                    trailingChevron: !recentDocs.isEmpty ? isExpanded : nil
                )
            }
            .buttonStyle(.plain)

            // Recent document rows — nested one level deeper
            if isExpanded {
                VStack(alignment: .leading, spacing: 0) {
                    ForEach(recentDocs) { doc in
                        Button {
                            onOpenDocument(doc)
                        } label: {
                            DocumentSidebarRow(artifact: doc)
                        }
                        .buttonStyle(.plain)
                    }
                }
            }
        }
    }
}

// MARK: – Document sidebar row (nested under Writing)

private struct DocumentSidebarRow: View {
    let artifact: ArtifactSummary
    @State private var isHovering = false

    var body: some View {
        HStack(spacing: 0) {
            // Extra indent beyond sub-row (18 + 14 + 8 = 40 base, add 16 more)
            Color.clear.frame(width: 34)

            Image(systemName: "doc.text")
                .font(.system(size: 11))
                .foregroundStyle(.tertiary)
                .frame(width: 14, alignment: .center)

            Color.clear.frame(width: 8)

            Text(artifact.label)
                .font(.system(size: 13))
                .foregroundStyle(.secondary)
                .lineLimit(1)

            Spacer(minLength: 8)
        }
        .padding(.vertical, 4)
        .frame(maxWidth: .infinity)
        .background(isHovering ? Color.primary.opacity(0.06) : Color.clear)
        .contentShape(Rectangle())
        .onHover { isHovering = $0 }
        .animation(.easeOut(duration: 0.12), value: isHovering)
    }
}

// MARK: – Artifact sub-row

private struct ArtifactSubRow: View {
    let title: String
    let systemImage: String
    let count: Int?
    var isAction: Bool = false
    var isSelected: Bool = false
    var trailingChevron: Bool? = nil   // nil = hidden, true = rotated (expanded), false = collapsed
    @State private var isHovering = false

    // Indent breakdown: 18pt margin + 14pt icon frame + 8pt gap = 40pt → aligns with parent text
    var body: some View {
        HStack(spacing: 0) {
            Color.clear.frame(width: 18)

            Image(systemName: systemImage)
                .font(.system(size: 12))
                .foregroundStyle(isSelected ? Color.accentColor : .secondary)
                .frame(width: 14, alignment: .center)

            Color.clear.frame(width: 8)

            Text(title)
                .font(.system(size: 14))
                .foregroundStyle(isAction || isSelected ? Color.accentColor : .primary)
                .lineLimit(1)

            Spacer(minLength: 8)

            if let count, count > 0 {
                Text("\(count)")
                    .font(.caption2.monospacedDigit())
                    .foregroundStyle(.secondary)
                    .padding(.horizontal, 5)
                    .padding(.vertical, 2)
                    .background(.secondary.opacity(0.12), in: Capsule())
                    .padding(.trailing, trailingChevron == nil ? 10 : 4)
            }

            if let expanded = trailingChevron {
                Image(systemName: "chevron.down")
                    .font(.system(size: 10, weight: .medium))
                    .foregroundStyle(.secondary)
                    .rotationEffect(.degrees(expanded ? 180 : 0))
                    .padding(.trailing, 10)
            }
        }
        .padding(.vertical, 5)
        .frame(maxWidth: .infinity)
        // Background goes here — after frame but before any outer padding,
        // so it fills the full row width
        .background(
            isSelected
                ? Color.accentColor.opacity(0.10)
                : (isHovering ? Color.primary.opacity(0.08) : Color.clear)
        )
        .contentShape(Rectangle())
        .onHover { isHovering = $0 }
        .animation(.easeOut(duration: 0.12), value: isHovering)
    }
}

// MARK: – Source row

private struct SourceRow: View {
    let source: SourceSummary
    private var isActive: Bool { source.syncState == "active" }

    var body: some View {
        HStack(spacing: 10) {
            ZStack {
                if isActive {
                    Circle()
                        .fill(Color.green.opacity(0.22))
                        .frame(width: 14, height: 14)
                }
                Circle()
                    .fill(isActive ? Color.green : Color.secondary.opacity(0.35))
                    .frame(width: 7, height: 7)
            }
            .frame(width: 20)

            VStack(alignment: .leading, spacing: 2) {
                Text(source.name)
                    .font(.system(size: 14))
                    .lineLimit(1)

                Text(source.type.capitalized)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Spacer()
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
    }
}

// MARK: – Empty / loading row

private struct SidebarEmptyRow: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.system(size: 14))
            .foregroundStyle(.secondary)
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
    }
}

// MARK: – Header

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
        .padding(.top, 16)
        .padding(.bottom, 12)
    }
}

// MARK: – Status footer

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

// MARK: – ArtifactSummary helpers

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
