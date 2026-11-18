import os

struct AppsLaunchRequest : Codable {
    let bundleId: String
}

@MainActor
struct AppsLaunchMethodHandler: RPCMethodHandler {
    static let methodName = "device.apps.launch"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(AppsLaunchRequest.self, from: params)

        let start = Date()

        logger.info("[Start] Launching app with bundle ID: \(request.bundleId)")
        XCUIApplication(bundleIdentifier: request.bundleId).activate()
        logger.info("[Done] Launching app with bundle ID: \(request.bundleId)")

        let duration = Date().timeIntervalSince(start)
        logger.info("Launch App duration took \(duration)")
        return .object(["success": .bool(true)])
    }
}

