import Foundation
import os

struct VoiceOverSetRequest: Codable {
    let enabled: Bool
}

@MainActor
struct VoiceOverSetMethodHandler: RPCMethodHandler {
    static let methodName = "device.voiceover.set"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(VoiceOverSetRequest.self, from: params)
        #if compiler(>=6.4)
        guard #available(iOS 27.0, *) else { throw VoiceOver.unavailable }

        let service = VoiceOver.service
        if request.enabled {
            try VoiceOver.perform("enable VoiceOver") { try service.enable() }
        } else {
            try VoiceOver.perform("disable VoiceOver") { try service.disable() }
        }

        logger.info("VoiceOver set to \(request.enabled), now \(service.isEnabled)")
        return .object(["enabled": .bool(service.isEnabled)])
        #else
        _ = request
        throw VoiceOver.unsupportedBuild
        #endif
    }
}
