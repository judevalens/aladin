import SwiftUI

enum HomeSection: String, CaseIterable, Identifiable {
    case artifacts
    case graph
    case feed
    case insights

    var id: String { rawValue }

    var title: String {
        switch self {
        case .artifacts: return "Artifacts"
        case .graph: return "Graph"
        case .feed: return "Feed"
        case .insights: return "Insights"
        }
    }

    var systemImage: String {
        switch self {
        case .artifacts: return "text.rectangle.page"
        case .graph: return "point.3.connected.trianglepath.dotted"
        case .feed: return "newspaper"
        case .insights: return "sparkles"
        }
    }

    var subtitle: String {
        switch self {
        case .artifacts:
            return "Live artifact inventory from the Go API"
        case .graph:
            return "Interactive graph workspace"
        case .feed:
            return "Recent artifacts and output"
        case .insights:
            return "Candidate insight review"
        }
    }
}
