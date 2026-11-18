import Foundation
import XCTest

/// Shared plumbing for the device.voiceover.* handlers, which drive XCUIDevice.voiceOverService.
/// The service only exists on iOS 27 and later, on simulators and real devices alike.
@MainActor
enum VoiceOver {
    static let unavailable = RPCMethodError.internalError("VoiceOver control requires iOS 27 or later")

    /// XCUIVoiceOverService ships in the iOS 27 SDK (Xcode 27, Swift 6.4). Built with an older Xcode the
    /// methods still exist but say why they can't work, since the runner embeds that Xcode's XCTest.
    static let unsupportedBuild = RPCMethodError.internalError(
        "VoiceOver control needs an agent built with Xcode 27 or later"
    )

    #if compiler(>=6.4)
    @available(iOS 27.0, *)
    static var service: XCUIVoiceOverService {
        XCUIDevice.shared.voiceOverService
    }

    /// Reading speech or moving the cursor before enable() fails with a vague "No speech available",
    /// so say what to do instead.
    @available(iOS 27.0, *)
    static func requireEnabled() throws {
        guard service.isEnabled else {
            throw RPCMethodError.internalError("VoiceOver is not enabled, call device.voiceover.set first")
        }
    }
    #endif

    /// Runs a service call, keeping the service's own message, which already names the cause
    /// (e.g. "moveIn() requires VoiceOver Navigation Style to be set to Groups.").
    static func perform<T>(_ action: String, _ body: () throws -> T) throws -> T {
        do {
            return try body()
        } catch {
            throw RPCMethodError.internalError("Failed to \(action): \(error.localizedDescription)")
        }
    }
}
