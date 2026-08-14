import SwiftUI
import Shared

@main
struct iOSApp: App {
    init() {
        KoinIosKt.startKoinIos()
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}
