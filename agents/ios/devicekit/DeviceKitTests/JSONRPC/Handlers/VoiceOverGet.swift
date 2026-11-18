import Foundation
import os

@MainActor
struct VoiceOverGetMethodHandler: RPCMethodHandler {
    static let methodName = "device.voiceover.get"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        #if compiler(>=6.4)
        guard #available(iOS 27.0, *) else { throw VoiceOver.unavailable }

        let enabled = VoiceOver.service.isEnabled
        logger.info("VoiceOver enabled: \(enabled)")
        return .object(["enabled": .bool(enabled)])
        #else
        throw VoiceOver.unsupportedBuild
        #endif
    }
}
