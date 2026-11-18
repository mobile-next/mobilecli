import ReplayKit
import SwiftUI

struct BroadcastStarterView: View {
    let preferredExtension =
        "com.mobilenext.devicekit-ios.BroadcastUploadExtension"

    var body: some View {
        BroadcastPickerView(preferredExtension: preferredExtension)
            .frame(width: 80, height: 80)
            .onAppear {
                autoTapPickerButton()
            }
    }

    private func autoTapPickerButton() {
        DispatchQueue.main.async {
            guard
                let windowScene = UIApplication.shared.connectedScenes.first
                    as? UIWindowScene,
                let window = windowScene.windows.first,
                let picker = findPicker(in: window)
            else {
                return
            }

            if let button = picker.subviews.first as? UIButton {
                button.sendActions(for: .touchUpInside)
            }
        }
    }

    private func findPicker(in view: UIView) -> RPSystemBroadcastPickerView? {
        if let picker = view as? RPSystemBroadcastPickerView {
            return picker
        }
        for subview in view.subviews {
            if let found = findPicker(in: subview) {
                return found
            }
        }
        return nil
    }
}
