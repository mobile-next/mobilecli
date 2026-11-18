import Foundation
import os

@MainActor
struct ClipboardGetMethodHandler: RPCMethodHandler {
    static let methodName = "device.clipboard.get"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let text = try await Pasteboard.withRunnerInForeground {
            try await Pasteboard.readText()
        }

        logger.info("Clipboard returned \(text.count) character(s)")
        return .object(["text": .string(text)])
    }
}
