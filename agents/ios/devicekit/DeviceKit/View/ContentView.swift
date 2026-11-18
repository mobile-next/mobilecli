import SwiftUI

struct ContentView: View {
    var body: some View {
        VStack(spacing: 12) {
            BroadcastStarterView()
            Text("Press to Start Broadcasting")
                .font(.footnote)
                .foregroundColor(.white.opacity(0.8))
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.black)
    }
}
