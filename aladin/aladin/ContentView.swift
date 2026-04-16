//
//  ContentView.swift
//  aladin
//
//  Created by Jude Paulemon on 4/8/26.
//

import SwiftUI

extension Notification.Name {
    static let aladinOpenNewNoteTab = Notification.Name("aladinOpenNewNoteTab")
    static let aladinOpenArtifactFilter = Notification.Name("aladinOpenArtifactFilter")
    static let aladinOpenDocument = Notification.Name("aladinOpenDocument")
}

struct ContentView: View {
    @StateObject private var model = AppModel()
    @State private var selectedSection: HomeSection? = .artifacts
    @State private var searchText = ""

    var body: some View {
        NavigationSplitView {
            Sidebar(
                selectedSection: $selectedSection,
                artifacts: model.artifacts,
                sources: model.sources,
                workerStatus: model.workerStatus,
                isLoading: model.isLoading,
                onNewNote: {
                    selectedSection = .artifacts
                    NotificationCenter.default.post(name: .aladinOpenNewNoteTab, object: nil)
                },
                onSelectFilter: { filterName in
                    selectedSection = .artifacts
                    NotificationCenter.default.post(name: .aladinOpenArtifactFilter, object: filterName)
                },
                onOpenDocument: { artifact in
                    selectedSection = .artifacts
                    NotificationCenter.default.post(name: .aladinOpenDocument, object: artifact.id)
                }
            )
            .navigationSplitViewColumnWidth(min: 250, ideal: 320, max: 380)
        } detail: {
            DetailView(
                selectedSection: $selectedSection,
                searchText: $searchText,
                model: model
            )
        }
        .navigationSplitViewStyle(.balanced)
        .task {
            await model.refresh()
        }
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
    }
}
