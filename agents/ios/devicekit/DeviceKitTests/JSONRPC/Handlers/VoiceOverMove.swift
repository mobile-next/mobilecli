import Foundation
import os

struct VoiceOverMoveRequest: Codable {
    let direction: String
}

/// Moves the VoiceOver cursor the way a swipe or two-finger scrub would, and returns what VoiceOver
/// speaks for the element it lands on.
@MainActor
struct VoiceOverMoveMethodHandler: RPCMethodHandler {
    static let methodName = "device.voiceover.move"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier ?? "devicekit-ios",
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(VoiceOverMoveRequest.self, from: params)
        guard ["next", "previous", "in", "out"].contains(request.direction) else {
            throw RPCMethodError.invalidParams(
                "Invalid direction '\(request.direction)', must be 'next', 'previous', 'in' or 'out'"
            )
        }
        #if compiler(>=6.4)
        guard #available(iOS 27.0, *) else { throw VoiceOver.unavailable }

        try VoiceOver.requireEnabled()
        let service = VoiceOver.service
        let output = try VoiceOver.perform("move VoiceOver \(request.direction)") {
            switch request.direction {
            case "next": return try service.moveForward()
            case "previous": return try service.moveBackward()
            case "in": return try service.moveIn()
            default: return try service.moveOut()
            }
        }
        let utterance = output.utterance
        logger.info("VoiceOver moved \(request.direction): \(utterance)")
        return .object(["utterance": .string(utterance)])
        #else
        throw VoiceOver.unsupportedBuild
        #endif
    }
}
