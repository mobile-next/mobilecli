import os

struct DeviceSettingsApplyRequest: Codable {
    let appearance: String?
}

@MainActor
struct DeviceSettingsApplyMethodHandler: RPCMethodHandler {
    static let methodName = "device.settings.apply"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(DeviceSettingsApplyRequest.self, from: params)

        if let appearance = request.appearance {
            try setAppearance(appearance)
        }

        return .object(["success": .bool(true)])
    }

    private func setAppearance(_ appearance: String) throws {
        let mode: XCUIDevice.Appearance
        switch appearance {
        case "light":
            mode = .light
        case "dark":
            mode = .dark
        default:
            throw RPCMethodError.invalidParams("Invalid appearance '\(appearance)', must be 'light' or 'dark'")
        }

        guard #available(iOS 15.0, *) else {
            throw RPCMethodError.internalError("Changing appearance requires iOS 15 or later")
        }

        logger.info("Setting appearance to: \(appearance)")
        XCUIDevice.shared.appearance = mode
    }
}
