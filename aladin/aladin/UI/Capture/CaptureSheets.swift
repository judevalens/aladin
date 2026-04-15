import AppKit
import SwiftUI
import WebKit

struct LinkCaptureSheet: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject var model: AppModel
    @State private var title = ""
    @State private var url = ""
    @State private var note = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Capture Link")
                .font(.title3.weight(.semibold))

            TextField("URL", text: $url)
            TextField("Title", text: $title)
            TextEditor(text: $note)
                .frame(height: 140)
                .overlay(RoundedRectangle(cornerRadius: 8).stroke(.quaternary))

            HStack {
                Spacer()

                Button("Cancel") {
                    dismiss()
                }

                Button("Save") {
                    Task {
                        let created = await model.createLink(title: title, url: url, note: note)
                        if created {
                            dismiss()
                        }
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(url.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(20)
        .frame(width: 460)
    }
}

struct WritingNodeCaptureSheet: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject var model: AppModel

    var body: some View {
        WritingNodeComposerView(
            model: model,
            onCancel: { dismiss() },
            onSaved: { dismiss() }
        )
        .frame(width: 960, height: 780)
    }
}

struct WritingNodeComposerView: View {
    @ObservedObject var model: AppModel
    let onCancel: () -> Void
    let onSaved: () -> Void
    @State private var title = ""
    @State private var content = ""

    var body: some View {
        WritingNodeEditorView(
            model: model,
            title: $title,
            content: $content,
            onCancel: onCancel,
            onSaved: onSaved
        )
    }
}

struct WritingNodeEditorView: View {
    @ObservedObject var model: AppModel
    @Binding var title: String
    @Binding var content: String
    let onCancel: () -> Void
    let onSaved: () -> Void
    @State private var isSaving = false
    @State private var editorCommand: MarkdownEditorCommand?

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

            MarkdownWebEditor(text: $content, command: $editorCommand)
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

    private func save() {
        Task {
            isSaving = true
            let created = await model.createWritingNode(title: normalizedTitle, content: content)
            isSaving = false
            if created {
                onSaved()
            }
        }
    }

    private func insert(_ snippet: String) {
        editorCommand = MarkdownEditorCommand(markdown: snippet)
    }
}

struct NoteComposeBar: View {
    let canSave: Bool
    let isSaving: Bool
    let onInsert: (String) -> Void
    let onCancel: () -> Void
    let onSave: () -> Void

    var body: some View {
        HStack(spacing: 6) {
            Group {
                NoteComposeBarButton("H2", help: "Heading") {
                    onInsert("\n## ")
                }

                NoteComposeBarButton("List", systemImage: "list.bullet", help: "Bullet list") {
                    onInsert("\n- ")
                }

                NoteComposeBarButton("Todo", systemImage: "checklist", help: "Checklist") {
                    onInsert("\n- [ ] ")
                }

                NoteComposeBarButton("Quote", systemImage: "quote.opening", help: "Quote") {
                    onInsert("\n> ")
                }

                NoteComposeBarButton("Audio", systemImage: "waveform", help: "Audio block") {
                    onInsert("\n![audio: Title](https://)\n")
                }
            }
            .disabled(isSaving)

            Spacer(minLength: 12)

            Button {
                onCancel()
            } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 12, weight: .medium))
            }
            .buttonStyle(NoteComposeBarControlStyle(width: 28))
            .keyboardShortcut(.cancelAction)
            .help("Close note")
            .disabled(isSaving)

            Button {
                onSave()
            } label: {
                if isSaving {
                    ProgressView()
                        .controlSize(.small)
                } else {
                    Text("Done")
                }
            }
            .buttonStyle(NoteComposeBarControlStyle(width: 48, isEmphasized: canSave))
            .keyboardShortcut(.defaultAction)
            .disabled(!canSave || isSaving)
        }
        .font(.callout.weight(.medium))
        .padding(.horizontal, 2)
        .frame(minHeight: 32)
    }
}

private struct NoteComposeBarButton: View {
    let title: String
    let systemImage: String?
    let help: String
    let action: () -> Void

    init(_ title: String, systemImage: String? = nil, help: String, action: @escaping () -> Void) {
        self.title = title
        self.systemImage = systemImage
        self.help = help
        self.action = action
    }

    var body: some View {
        Button(action: action) {
            if let systemImage {
                Label(title, systemImage: systemImage)
                    .labelStyle(.iconOnly)
            } else {
                Text(title)
                    .font(.caption.weight(.semibold))
            }
        }
        .buttonStyle(NoteComposeBarControlStyle(width: 32))
        .help(help)
    }
}

private struct NoteComposeBarControlStyle: ButtonStyle {
    let width: CGFloat
    var isEmphasized = false

    func makeBody(configuration: Configuration) -> some View {
        NoteComposeBarControlLabel(
            configuration: configuration,
            width: width,
            isEmphasized: isEmphasized
        )
    }
}

private struct NoteComposeBarControlLabel: View {
    let configuration: ButtonStyle.Configuration
    let width: CGFloat
    let isEmphasized: Bool
    @Environment(\.isEnabled) private var isEnabled
    @State private var isHovering = false

    var body: some View {
        configuration.label
            .frame(width: width, height: 28)
            .foregroundStyle(foregroundStyle)
            .background(
                RoundedRectangle(cornerRadius: 6)
                    .fill(backgroundColor)
            )
            .contentShape(RoundedRectangle(cornerRadius: 6))
            .opacity(isEnabled ? 1 : 0.45)
            .onHover { isHovering = $0 }
    }

    private var foregroundStyle: Color {
        if !isEnabled {
            return Color.secondary
        }
        if isEmphasized || isHovering || configuration.isPressed {
            return Color.primary
        }
        return Color.secondary
    }

    private var backgroundColor: Color {
        if configuration.isPressed {
            return Color.primary.opacity(0.11)
        }
        if isHovering && isEnabled {
            return Color.primary.opacity(0.07)
        }
        return Color.clear
    }
}

struct MarkdownEditorCommand: Equatable {
    let id = UUID()
    let markdown: String

    static func == (lhs: MarkdownEditorCommand, rhs: MarkdownEditorCommand) -> Bool {
        lhs.id == rhs.id
    }
}

struct MarkdownWebEditor: NSViewRepresentable {
    @Binding var text: String
    @Binding var command: MarkdownEditorCommand?

    func makeCoordinator() -> Coordinator {
        Coordinator(text: $text)
    }

    func makeNSView(context: Context) -> WKWebView {
        let contentController = WKUserContentController()
        contentController.add(context.coordinator, name: "markdownEditor")

        let configuration = WKWebViewConfiguration()
        configuration.userContentController = contentController

        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.setValue(false, forKey: "drawsBackground")
        webView.navigationDelegate = context.coordinator
        context.coordinator.webView = webView
        context.coordinator.loadEditorHTML()
        return webView
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        context.coordinator.webView = nsView
        context.coordinator.pendingText = text
        guard context.coordinator.isLoaded else { return }
        if context.coordinator.lastSentText != text {
            context.coordinator.push(text: text)
        }
        if context.coordinator.lastCommand != command {
            context.coordinator.lastCommand = command
            if let command {
                context.coordinator.insert(markdown: command.markdown)
            }
        }
    }

    final class Coordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
        @Binding private var text: String
        weak var webView: WKWebView?
        var isLoaded = false
        var lastSentText = ""
        var pendingText = ""
        var lastCommand: MarkdownEditorCommand?

        init(text: Binding<String>) {
            _text = text
        }

        func loadEditorHTML() {
            guard let webView,
                  let fileURL = Bundle.main.url(forResource: "markdown_editor", withExtension: "html")
            else { return }

            webView.loadFileURL(fileURL, allowingReadAccessTo: fileURL.deletingLastPathComponent())
        }

        func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
            guard message.name == "markdownEditor",
                  let body = message.body as? [String: Any],
                  let type = body["type"] as? String,
                  type == "change",
                  let content = body["content"] as? String
            else { return }

            lastSentText = content
            if text != content {
                text = content
            }
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            isLoaded = true
            push(text: pendingText)
        }

        func push(text: String) {
            guard let webView else { return }
            lastSentText = text
            let payload = Self.javaScriptEscaped(text)
            webView.evaluateJavaScript("window.setMarkdown(\(payload));", completionHandler: nil)
        }

        func insert(markdown: String) {
            guard let webView else { return }
            let payload = Self.javaScriptEscaped(markdown)
            webView.evaluateJavaScript("window.insertMarkdown(\(payload));", completionHandler: nil)
        }

        private static func javaScriptEscaped(_ string: String) -> String {
            let data = try? JSONSerialization.data(withJSONObject: [string], options: [])
            let encoded = data.flatMap { String(data: $0, encoding: .utf8) } ?? "[\"\"]"
            return String(encoded.dropFirst().dropLast())
        }
    }
}

struct SharedNoteDocument: Equatable {
    let id: String
    let markdown: String
}

struct SharedNoteEditorCommand: Equatable {
    enum Action: Equatable {
        case insert(String)
        case close
    }

    let id = UUID()
    let documentID: String
    let action: Action

    static func == (lhs: SharedNoteEditorCommand, rhs: SharedNoteEditorCommand) -> Bool {
        lhs.id == rhs.id
    }
}

struct SharedNoteEditorWebView: NSViewRepresentable {
    let documents: [SharedNoteDocument]
    let activeDocumentID: String?
    @Binding var command: SharedNoteEditorCommand?
    let onChange: (String, String) -> Void

    func makeCoordinator() -> Coordinator {
        Coordinator(onChange: onChange)
    }

    func makeNSView(context: Context) -> WKWebView {
        let contentController = WKUserContentController()
        contentController.add(context.coordinator, name: "markdownEditor")

        let configuration = WKWebViewConfiguration()
        configuration.userContentController = contentController

        let webView = WKWebView(frame: .zero, configuration: configuration)
        webView.setValue(false, forKey: "drawsBackground")
        webView.navigationDelegate = context.coordinator
        context.coordinator.webView = webView
        context.coordinator.loadEditorHTML()
        return webView
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        context.coordinator.webView = nsView
        context.coordinator.pendingDocuments = documents
        context.coordinator.pendingActiveDocumentID = activeDocumentID
        guard context.coordinator.isLoaded else { return }

        context.coordinator.sync(documents: documents, activeDocumentID: activeDocumentID)

        if context.coordinator.lastCommand != command {
            context.coordinator.lastCommand = command
            if let command {
                context.coordinator.run(command: command)
            }
        }
    }

    final class Coordinator: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
        weak var webView: WKWebView?
        var isLoaded = false
        var pendingDocuments: [SharedNoteDocument] = []
        var pendingActiveDocumentID: String?
        var lastKnownMarkdown: [String: String] = [:]
        var knownDocumentIDs = Set<String>()
        var lastActiveDocumentID: String?
        var lastCommand: SharedNoteEditorCommand?
        let onChange: (String, String) -> Void

        init(onChange: @escaping (String, String) -> Void) {
            self.onChange = onChange
        }

        func loadEditorHTML() {
            guard let webView,
                  let fileURL = Bundle.main.url(forResource: "markdown_editor", withExtension: "html")
            else { return }

            webView.loadFileURL(fileURL, allowingReadAccessTo: fileURL.deletingLastPathComponent())
        }

        func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
            guard message.name == "markdownEditor",
                  let body = message.body as? [String: Any],
                  let type = body["type"] as? String,
                  type == "change",
                  let id = body["id"] as? String,
                  let content = body["content"] as? String
            else { return }

            lastKnownMarkdown[id] = content
            onChange(id, content)
        }

        func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
            isLoaded = true
            evaluate("window.setEditorCacheLimit(5);")
            sync(documents: pendingDocuments, activeDocumentID: pendingActiveDocumentID)
        }

        func sync(documents: [SharedNoteDocument], activeDocumentID: String?) {
            let nextIDs = Set(documents.map(\.id))
            for closedID in knownDocumentIDs.subtracting(nextIDs) {
                close(documentID: closedID)
            }

            for document in documents {
                if knownDocumentIDs.contains(document.id) {
                    if lastKnownMarkdown[document.id] != document.markdown {
                        update(document: document)
                    }
                } else {
                    open(document: document)
                }
            }

            knownDocumentIDs = nextIDs

            if let activeDocumentID, activeDocumentID != lastActiveDocumentID {
                activate(documentID: activeDocumentID)
                lastActiveDocumentID = activeDocumentID
            }
        }

        func run(command: SharedNoteEditorCommand) {
            switch command.action {
            case .insert(let markdown):
                let id = Self.javaScriptEscaped(command.documentID)
                let payload = Self.javaScriptEscaped(markdown)
                evaluate("window.insertMarkdown(\(id), \(payload));")
            case .close:
                close(documentID: command.documentID)
            }
        }

        private func open(document: SharedNoteDocument) {
            lastKnownMarkdown[document.id] = document.markdown
            let payload = Self.javaScriptObject([
                "id": document.id,
                "markdown": document.markdown
            ])
            evaluate("window.openEditor(\(payload));")
        }

        private func update(document: SharedNoteDocument) {
            lastKnownMarkdown[document.id] = document.markdown
            let id = Self.javaScriptEscaped(document.id)
            let markdown = Self.javaScriptEscaped(document.markdown)
            evaluate("window.updateEditor(\(id), \(markdown));")
        }

        private func activate(documentID: String) {
            let id = Self.javaScriptEscaped(documentID)
            evaluate("window.activateEditor(\(id));")
        }

        private func close(documentID: String) {
            knownDocumentIDs.remove(documentID)
            lastKnownMarkdown.removeValue(forKey: documentID)
            if lastActiveDocumentID == documentID {
                lastActiveDocumentID = nil
            }
            let id = Self.javaScriptEscaped(documentID)
            evaluate("window.closeEditor(\(id));")
        }

        private func evaluate(_ script: String) {
            webView?.evaluateJavaScript(script, completionHandler: nil)
        }

        private static func javaScriptObject(_ object: [String: String]) -> String {
            let data = try? JSONSerialization.data(withJSONObject: object, options: [])
            return data.flatMap { String(data: $0, encoding: .utf8) } ?? "{}"
        }

        private static func javaScriptEscaped(_ string: String) -> String {
            let data = try? JSONSerialization.data(withJSONObject: [string], options: [])
            let encoded = data.flatMap { String(data: $0, encoding: .utf8) } ?? "[\"\"]"
            return String(encoded.dropFirst().dropLast())
        }
    }
}

struct VoiceNoteCaptureSheet: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject var model: AppModel
    @StateObject private var recorder = VoiceRecorder()
    @State private var title = ""
    @State private var transcript = ""
    @State private var isSaving = false

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Capture Voice Note")
                .font(.title3.weight(.semibold))

            TextField("Title", text: $title)
                .textFieldStyle(.roundedBorder)

            VStack(spacing: 18) {
                Text(recorder.isRecording ? "Recording in progress" : "Ready to record")
                    .font(.headline)

                Text(durationText)
                    .font(.system(.title3, design: .monospaced).weight(.semibold))
                    .foregroundStyle(recorder.isRecording ? .primary : .secondary)

                Button {
                    if recorder.isRecording {
                        recorder.stopRecording()
                    } else {
                        Task {
                            await recorder.startRecording()
                        }
                    }
                } label: {
                    ZStack {
                        Circle()
                            .fill(recorder.isRecording ? Color.red.opacity(0.2) : Color.accentColor.opacity(0.12))
                            .frame(width: 148, height: 148)
                            .scaleEffect(outerRingScale)
                            .opacity(outerRingOpacity)

                        Circle()
                            .fill(recorder.isRecording ? Color.red : Color.accentColor)
                            .frame(width: 108, height: 108)
                            .scaleEffect(innerCircleScale)
                            .shadow(color: recorder.isRecording ? Color.red.opacity(0.35 + (0.3 * recorder.inputLevel)) : .clear, radius: 18)
                            .overlay {
                                Image(systemName: recorder.isRecording ? "stop.fill" : "mic.fill")
                                    .font(.system(size: 34, weight: .semibold))
                                    .foregroundStyle(.white)
                            }
                    }
                    .frame(maxWidth: .infinity)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .disabled(isSaving || recorder.permissionState == .denied)

                Text(recorder.isRecording ? "Tap to stop" : (recorder.recordedFileURL == nil ? "Tap to start recording" : "Tap to record again"))
                    .font(.subheadline)
                    .foregroundStyle(.secondary)

                Label(recorder.statusText, systemImage: recorder.statusSymbol)
                    .foregroundStyle(recorder.isRecording ? .red : .secondary)

                if let recordedFileURL = recorder.recordedFileURL {
                    LabeledContent("Captured File", value: recordedFileURL.lastPathComponent)
                        .font(.caption)
                        .foregroundStyle(.secondary)

                    HStack(spacing: 12) {
                        Button {
                            recorder.togglePlayback()
                        } label: {
                            Label(previewButtonTitle, systemImage: previewButtonSymbol)
                        }
                        .buttonStyle(.bordered)

                        Text(playbackTimeText)
                            .font(.system(.body, design: .monospaced))
                            .foregroundStyle(.secondary)
                    }

                    Slider(
                        value: Binding(
                            get: { scrubberValue },
                            set: { newValue in
                                recorder.seek(to: newValue * recorder.recordedDuration)
                            }
                        ),
                        in: 0...1,
                        onEditingChanged: { _ in }
                    )
                    .tint(.accentColor)
                }

                if let errorMessage = recorder.errorMessage {
                    Text(errorMessage)
                        .font(.caption)
                        .foregroundStyle(.red)
                }

                if recorder.permissionState == .denied {
                    Text("Microphone access is denied. Enable it in System Settings to capture voice notes.")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
            }
            .frame(maxWidth: .infinity)
            .padding(.vertical, 12)

            VStack(alignment: .leading, spacing: 8) {
                Text("Notes or Transcript")
                    .font(.headline)

                TextEditor(text: $transcript)
                    .frame(height: 140)
                    .overlay(RoundedRectangle(cornerRadius: 8).stroke(.quaternary))
            }

            HStack {
                Spacer()

                Button("Cancel") {
                    recorder.discardRecording()
                    dismiss()
                }

                Button("Save Voice Note") {
                    Task {
                        guard let fileURL = recorder.recordedFileURL else { return }
                        isSaving = true
                        let created = await model.createVoiceNote(fileURL: fileURL, title: title, transcript: transcript)
                        isSaving = false
                        if created {
                            recorder.discardRecording()
                            dismiss()
                        }
                    }
                }
                .buttonStyle(.borderedProminent)
                .disabled(recorder.recordedFileURL == nil || recorder.isRecording || isSaving)
            }
        }
        .padding(20)
        .frame(width: 520)
        .animation(.easeOut(duration: 0.12), value: recorder.inputLevel)
        .animation(.easeOut(duration: 0.2), value: recorder.isRecording)
        .animation(.easeOut(duration: 0.12), value: recorder.elapsedDuration)
        .onDisappear {
            if !isSaving {
                recorder.discardRecording()
            }
        }
    }

    private var outerRingScale: CGFloat {
        if recorder.isRecording {
            return 0.96 + (recorder.inputLevel * 0.32)
        }
        return 0.92
    }

    private var outerRingOpacity: Double {
        if recorder.isRecording {
            return 0.35 + (Double(recorder.inputLevel) * 0.45)
        }
        return 1
    }

    private var innerCircleScale: CGFloat {
        if recorder.isRecording {
            return 1 + (recorder.inputLevel * 0.08)
        }
        return 1
    }

    private var durationText: String {
        formattedTime(recorder.isRecording ? recorder.elapsedDuration : recorder.recordedDuration)
    }

    private var playbackTimeText: String {
        "\(formattedTime(currentPlaybackTime)) / \(formattedTime(recorder.recordedDuration))"
    }

    private func formattedTime(_ duration: TimeInterval) -> String {
        let totalSeconds = max(0, Int(duration.rounded(.down)))
        let minutes = totalSeconds / 60
        let seconds = totalSeconds % 60
        return String(format: "%02d:%02d", minutes, seconds)
    }

    private var currentPlaybackTime: TimeInterval {
        recorder.elapsedDuration
    }

    private var scrubberValue: Double {
        guard recorder.recordedDuration > 0 else { return 0 }
        return min(max(currentPlaybackTime / recorder.recordedDuration, 0), 1)
    }

    private var previewButtonTitle: String {
        if recorder.isPlaying {
            return "Pause Preview"
        }
        if recorder.isPlaybackPaused {
            return currentPlaybackTime >= recorder.recordedDuration ? "Play Again" : "Resume Preview"
        }
        return "Play Preview"
    }

    private var previewButtonSymbol: String {
        if recorder.isPlaying {
            return "pause.fill"
        }
        return "play.fill"
    }
}
