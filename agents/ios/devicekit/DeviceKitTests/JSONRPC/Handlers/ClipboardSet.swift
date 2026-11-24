import Foundation
import os

struct ClipboardSetRequest: Codable {
    let text: String
}

@MainActor
struct ClipboardSetMethodHandler: RPCMethodHandler {
    static let methodName = "device.clipboard.set"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(ClipboardSetRequest.self, from: params)

        try await Pasteboard.withRunnerInForeground {
            try Pasteboard.write(request.text)
        }

        logger.info("Clipboard set to \(request.text.count) character(s)")
        return .object(["success": .bool(true)])
    }
}
