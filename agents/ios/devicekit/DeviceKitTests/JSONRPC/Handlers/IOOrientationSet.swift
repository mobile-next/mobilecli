import os

struct IOOrientationSetRequest: Codable {
    let orientation: String
}

@MainActor
struct IOOrientationSetMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.orientation.set"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOOrientationSetRequest.self, from: params)

        let target: UIDeviceOrientation
        switch request.orientation.uppercased() {
        case "PORTRAIT":
            target = .portrait
        case "LANDSCAPE":
            target = .landscapeLeft
        default:
            throw RPCMethodError.invalidParams("Invalid orientation '\(request.orientation)', must be 'PORTRAIT' or 'LANDSCAPE'")
        }

        let start = Date()
        logger.info("[Start] Setting orientation to: \(request.orientation)")
        XCUIDevice.shared.orientation = target
        let duration = Date().timeIntervalSince(start)
        logger.info("[Done] Setting orientation took \(duration)")

        return .object(["success": .bool(true)])
    }
}
