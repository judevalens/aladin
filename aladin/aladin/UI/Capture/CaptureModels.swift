import SwiftUI

enum CaptureOption: String, CaseIterable, Identifiable {
    case link
    case voiceNote
    case writingNode

    var id: String { rawValue }

    var title: String {
        switch self {
        case .link: return "Link"
        case .voiceNote: return "Voice Note"
        case .writingNode: return "New Note"
        }
    }

    var systemImage: String {
        switch self {
        case .link: return "link"
        case .voiceNote: return "waveform"
        case .writingNode: return "square.and.pencil"
        }
    }
}

enum CaptureSheet: Identifiable {
    case link
    case voiceNote
    case writingNode

    var id: String {
        switch self {
        case .link:
            return "link"
        case .voiceNote:
            return "voice-note"
        case .writingNode:
            return "writing-node"
        }
    }
}
