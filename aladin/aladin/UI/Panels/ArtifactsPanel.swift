import SwiftUI

struct ArtifactsPanel: View {
    let searchText: String
    @ObservedObject var viewModel: DetailViewModel
    @Binding var tabs: [ArtifactTab]
    @Binding var activeTabID: ArtifactTab.ID
    @State private var editorCommand: SharedNoteEditorCommand?

    private var filteredArtifacts: [ArtifactSummary] {
        viewModel.artifactService.filtered(by: searchText)
    }

    private var activeTab: ArtifactTab {
        tabs.first { $0.id == activeTabID } ?? .home
    }

    private var noteDocuments: [SharedNoteDocument] {
        tabs.compactMap { tab in
            guard tab.kind == .noteDraft else { return nil }
            return SharedNoteDocument(id: tab.id, markdown: tab.noteContent)
        }
    }

    private var activeNoteID: String? {
        activeTab.kind == .noteDraft ? activeTab.id : nil
    }

    private var activeFilterName: String? {
        if case .artifactFilter(let name) = activeTab.kind { return name }
        return nil
    }

    private var activeDocument: ArtifactSummary? {
        if case .document(let artifact) = activeTab.kind { return artifact }
        return nil
    }

    var body: some View {
        VStack(spacing: 0) {
            ArtifactTabBar(
                tabs: tabs,
                activeTabID: activeTabID,
                onSelect: { activeTabID = $0 },
                onClose: closeTab
            )

            Divider()

            ZStack {
                artifactOverview
                    .opacity(activeTab.kind == .home ? 1 : 0)
                    .allowsHitTesting(activeTab.kind == .home)

                if !noteDocuments.isEmpty {
                    SharedNoteEditorChrome(
                        viewModel: viewModel,
                        title: activeNoteTitleBinding,
                        content: activeNoteContentBinding,
                        documents: noteDocuments,
                        activeDocumentID: activeNoteID,
                        command: $editorCommand,
                        onChange: updateNoteContent,
                        onCancel: { closeTab(activeTab.id) },
                        onSaved: { closeTab(activeTab.id) }
                    )
                    .opacity(activeTab.kind == .noteDraft ? 1 : 0)
                    .allowsHitTesting(activeTab.kind == .noteDraft)
                }

                if let name = activeFilterName {
                    FilterPlaceholderView(filterName: name)
                }

                if let artifact = activeDocument {
                    DocumentView(artifact: artifact)
                }
            }
        }
    }

    private var activeNoteTitleBinding: Binding<String> {
        guard let index = tabs.firstIndex(where: { $0.id == activeTab.id }),
              tabs[index].kind == .noteDraft
        else { return .constant("") }
        return binding(for: index, keyPath: \.title)
    }

    private var activeNoteContentBinding: Binding<String> {
        guard let index = tabs.firstIndex(where: { $0.id == activeTab.id }),
              tabs[index].kind == .noteDraft
        else { return .constant("") }
        return binding(for: index, keyPath: \.noteContent)
    }

    private var artifactOverview: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                HeroCard(
                    artifactCount: viewModel.artifacts.count,
                    sourceCount: viewModel.sources.count,
                    queueCount: viewModel.workerStatus?.queue.pending ?? 0,
                    lastUpdated: viewModel.lastUpdated
                )

                if let errorMessage = viewModel.errorMessage {
                    ErrorBanner(message: errorMessage)
                }

                LazyVGrid(
                    columns: [
                        GridItem(.flexible(minimum: 220), spacing: 16),
                        GridItem(.flexible(minimum: 220), spacing: 16),
                        GridItem(.flexible(minimum: 220), spacing: 16)
                    ],
                    spacing: 16
                ) {
                    MetricCard(
                        title: "Artifacts",
                        value: "\(viewModel.artifacts.count)",
                        note: "Current top-level artifact inventory from the Go API."
                    )
                    MetricCard(
                        title: "Sources",
                        value: "\(viewModel.sources.count)",
                        note: "Active source inventory visible in the workspace shell."
                    )
                    MetricCard(
                        title: "Queue",
                        value: "\(viewModel.workerStatus?.queue.pendingPipeline ?? 0)",
                        note: "Pending pipeline backlog reported by the worker status endpoint."
                    )
                }

                VStack(alignment: .leading, spacing: 12) {
                    HStack {
                        Text("Recent Artifacts")
                            .font(.headline)

                        Spacer()

                        Text("\(filteredArtifacts.count) visible")
                            .font(.caption)
                            .foregroundStyle(.secondary)
                    }

                    if filteredArtifacts.isEmpty {
                        EmptyStateCard(
                            title: "No artifacts to show",
                            note: searchText.isEmpty
                                ? "The API returned an empty artifact list."
                                : "No artifacts matched your current search."
                        )
                    } else {
                        VStack(spacing: 10) {
                            ForEach(filteredArtifacts.prefix(40)) { artifact in
                                ArtifactRow(artifact: artifact)
                            }
                        }
                    }
                }
            }
            .padding(20)
        }
    }

    private func closeTab(_ id: ArtifactTab.ID) {
        guard let index = tabs.firstIndex(where: { $0.id == id }),
              tabs[index].isClosable
        else { return }
        let wasActive = tabs[index].id == activeTabID
        editorCommand = SharedNoteEditorCommand(documentID: id, action: .close)
        tabs.remove(at: index)

        guard wasActive else { return }
        if tabs.indices.contains(index) {
            activeTabID = tabs[index].id
        } else {
            activeTabID = tabs.last?.id ?? ArtifactTab.homeID
        }
    }

    private func binding<Value>(for index: Int, keyPath: WritableKeyPath<ArtifactTab, Value>) -> Binding<Value> {
        Binding(
            get: { tabs[index][keyPath: keyPath] },
            set: { tabs[index][keyPath: keyPath] = $0 }
        )
    }

    private func updateNoteContent(id: String, markdown: String) {
        guard let index = tabs.firstIndex(where: { $0.id == id }),
              tabs[index].kind == .noteDraft
        else { return }
        tabs[index].noteContent = markdown
    }
}

private struct ArtifactTabBar: View {
    let tabs: [ArtifactTab]
    let activeTabID: ArtifactTab.ID
    let onSelect: (ArtifactTab.ID) -> Void
    let onClose: (ArtifactTab.ID) -> Void

    var body: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 4) {
                ForEach(tabs) { tab in
                    ArtifactTabButton(
                        tab: tab,
                        isActive: tab.id == activeTabID,
                        onSelect: { onSelect(tab.id) },
                        onClose: { onClose(tab.id) }
                    )
                }
            }
            .padding(.horizontal, 14)
            .padding(.vertical, 7)
        }
        .frame(height: 44)
        .background(.bar)
    }
}

private struct ArtifactTabButton: View {
    let tab: ArtifactTab
    let isActive: Bool
    let onSelect: () -> Void
    let onClose: () -> Void
    @State private var isHovering = false

    var body: some View {
        Button(action: onSelect) {
            HStack(spacing: 8) {
                HStack(spacing: 7) {
                    Image(systemName: tab.tabSystemImage)
                    Text(tab.displayTitle)
                        .lineLimit(1)
                }
                .frame(maxWidth: 190, alignment: .leading)

                // Close button — intercepts its own taps without bubbling to the select button
                Button(action: onClose) {
                    Image(systemName: "xmark")
                        .font(.system(size: 10, weight: .semibold))
                        .frame(width: 16, height: 16)
                        .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .foregroundStyle(.secondary)
                .opacity(tab.isClosable && (isHovering || isActive) ? 1 : 0)
                .disabled(!tab.isClosable)
            }
            .font(.caption.weight(.medium))
            .foregroundStyle(isActive ? .primary : .secondary)
            .padding(.horizontal, 10)
            .frame(height: 30)
            .frame(maxWidth: .infinity)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(isActive ? Color.primary.opacity(0.08) : (isHovering ? Color.primary.opacity(0.05) : Color.clear))
        )
        .onHover { isHovering = $0 }
    }
}

private struct SharedNoteEditorChrome: View {
    @ObservedObject var viewModel: DetailViewModel
    @Binding var title: String
    @Binding var content: String
    let documents: [SharedNoteDocument]
    let activeDocumentID: String?
    @Binding var command: SharedNoteEditorCommand?
    let onChange: (String, String) -> Void
    let onCancel: () -> Void
    let onSaved: () -> Void
    @State private var isSaving = false

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            TextField("Untitled note", text: $title)
                .font(.system(size: 34, weight: .semibold))
                .textFieldStyle(.plain)
                .padding(.horizontal, 2)

            NoteComposeBar(
                canSave: canSave,
                isSaving: isSaving,
                onInsert: insert,
                onCancel: onCancel,
                onSave: save
            )

            SharedNoteEditorWebView(
                documents: documents,
                activeDocumentID: activeDocumentID,
                command: $command,
                onChange: onChange
            )
            .frame(minHeight: 520, maxHeight: .infinity)
        }
        .frame(maxWidth: 900, maxHeight: .infinity, alignment: .topLeading)
        .padding(.horizontal, 56)
        .padding(.top, 38)
        .padding(.bottom, 12)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color(nsColor: .controlBackgroundColor))
    }

    private var canSave: Bool {
        !content.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    private var normalizedTitle: String {
        let trimmedTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmedTitle.isEmpty ? "Untitled Note" : trimmedTitle
    }

    private func insert(_ snippet: String) {
        guard let activeDocumentID else { return }
        command = SharedNoteEditorCommand(documentID: activeDocumentID, action: .insert(snippet))
    }

    private func save() {
        Task {
            isSaving = true
            let created = await viewModel.createWritingNode(title: normalizedTitle, content: content)
            isSaving = false
            if created {
                onSaved()
            }
        }
    }
}

private struct HeroCard: View {
    let artifactCount: Int
    let sourceCount: Int
    let queueCount: Int
    let lastUpdated: Date?

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Label("Artifacts Workspace", systemImage: "text.rectangle.page")
                .font(.title2.weight(.semibold))

            Text("Inspect recent artifacts, source output, and worker pressure from the native shell while the graph and review surfaces stay separate.")
                .foregroundStyle(.secondary)

            HStack(spacing: 12) {
                Badge(text: "\(artifactCount) artifacts")
                Badge(text: "\(sourceCount) sources")
                Badge(text: "\(queueCount) active cycles")
            }

            if let lastUpdated {
                Text("Last updated \(lastUpdated.formatted(date: .omitted, time: .standard))")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(24)
        .background(.background, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).stroke(.primary.opacity(0.07), lineWidth: 0.5))
        .shadow(color: .black.opacity(0.05), radius: 8, y: 2)
    }
}

private struct Badge: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.caption.weight(.medium))
            .foregroundStyle(Color.accentColor)
            .padding(.horizontal, 10)
            .padding(.vertical, 6)
            .background(Color.accentColor.opacity(0.10), in: Capsule())
            .overlay(Capsule().stroke(Color.accentColor.opacity(0.25), lineWidth: 0.5))
    }
}

struct MetricCard: View {
    let title: String
    let value: String
    let note: String

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title)
                .font(.caption)
                .foregroundStyle(.secondary)
            Text(value)
                .font(.title3.weight(.semibold))
            Text(note)
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, minHeight: 120, alignment: .topLeading)
        .padding(18)
        .background(.background, in: RoundedRectangle(cornerRadius: 16, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 16, style: .continuous).stroke(.primary.opacity(0.07), lineWidth: 0.5))
        .shadow(color: .black.opacity(0.05), radius: 8, y: 2)
    }
}

private struct ArtifactRow: View {
    let artifact: ArtifactSummary
    @State private var isHovering = false

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(alignment: .firstTextBaseline) {
                VStack(alignment: .leading, spacing: 3) {
                    Text(artifact.label)
                        .font(.headline)
                    Text(artifact.type.capitalized)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Spacer()

                Text(artifact.createdAt)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            if !artifact.content.isEmpty {
                Text(artifact.content)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .lineLimit(3)
            }

            HStack(spacing: 12) {
                if let sourceURL = artifact.sourceURL, !sourceURL.isEmpty {
                    Label(sourceURL, systemImage: "link")
                        .lineLimit(1)
                }

                if artifact.childCount > 0 {
                    Label("\(artifact.childCount) children", systemImage: "square.stack.3d.down.right")
                }
            }
            .font(.caption)
            .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(16)
        .background(.background, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(.primary.opacity(0.07), lineWidth: 0.5))
        .shadow(color: .black.opacity(isHovering ? 0.08 : 0.04), radius: isHovering ? 12 : 6, y: isHovering ? 3 : 1)
        .animation(.easeOut(duration: 0.15), value: isHovering)
        .onHover { isHovering = $0 }
    }
}

private struct ErrorBanner: View {
    let message: String

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "exclamationmark.triangle.fill")
                .foregroundStyle(.orange)
            Text(message)
                .font(.subheadline)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(14)
        .background(Color.orange.opacity(0.12), in: RoundedRectangle(cornerRadius: 12, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 12, style: .continuous).stroke(Color.orange.opacity(0.25), lineWidth: 0.5))
        .shadow(color: .orange.opacity(0.10), radius: 8, y: 2)
    }
}

private struct DocumentView: View {
    let artifact: ArtifactSummary

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                VStack(alignment: .leading, spacing: 6) {
                    Text(artifact.label)
                        .font(.title2.weight(.semibold))

                    Text(artifact.createdAt)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }

                Divider()

                if artifact.content.isEmpty {
                    Text("No content yet.")
                        .font(.body)
                        .foregroundStyle(.secondary)
                } else {
                    Text(artifact.content)
                        .font(.body)
                        .lineSpacing(5)
                }
            }
            .padding(48)
            .frame(maxWidth: 720, alignment: .leading)
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .background(Color(nsColor: .controlBackgroundColor))
    }
}

private struct FilterPlaceholderView: View {
    let filterName: String

    private var systemImage: String {
        switch filterName {
        case "Inbox":   return "tray"
        case "Writing": return "square.and.pencil"
        case "Links":   return "link"
        case "Audio":   return "waveform"
        default:        return "folder"
        }
    }

    var body: some View {
        VStack(spacing: 16) {
            Image(systemName: systemImage)
                .font(.system(size: 36, weight: .light))
                .foregroundStyle(.secondary)

            VStack(spacing: 6) {
                Text(filterName)
                    .font(.title3.weight(.semibold))

                Text("This filter view is coming soon.")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color(nsColor: .controlBackgroundColor))
    }
}

private struct EmptyStateCard: View {
    let title: String
    let note: String

    var body: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(title)
                .font(.headline)
            Text(note)
                .font(.subheadline)
                .foregroundStyle(.secondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(18)
        .background(.background, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).stroke(.primary.opacity(0.07), lineWidth: 0.5))
        .shadow(color: .black.opacity(0.04), radius: 6, y: 1)
    }
}
