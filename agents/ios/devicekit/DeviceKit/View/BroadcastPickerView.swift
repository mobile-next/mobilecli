import ReplayKit
import SwiftUI

struct BroadcastPickerView: UIViewRepresentable {
    let preferredExtension: String

    func makeUIView(context: Context) -> RPSystemBroadcastPickerView {
        let picker = RPSystemBroadcastPickerView(
            frame: CGRect(
                x: 0,
                y: 0,
                width: 80,
                height: 80
            )
        )
        picker.preferredExtension = preferredExtension

        if let button = picker.subviews.first as? UIButton {
            button.imageView?.tintColor = .white
        }

        return picker
    }

    func updateUIView(_ uiView: RPSystemBroadcastPickerView, context: Context) {
    }
}
