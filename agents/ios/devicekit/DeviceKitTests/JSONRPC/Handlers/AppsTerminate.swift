import os

struct AppsTerminateRequest : Codable {
    let bundleId: String
}

@MainActor
struct AppsTerminateMethodHandler: RPCMethodHandler {
    static let methodName = "device.apps.terminate"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(AppsTerminateRequest.self, from: params)

        let start = Date()

        let app = XCUIApplication(bundleIdentifier: request.bundleId)
        let wasRunning = app.running
        if wasRunning {
            logger.info("[Start] Terminating app with bundle ID: \(request.bundleId)")
            app.terminate()
        }

        let duration = Date().timeIntervalSince(start)
        logger.info("[Done] Terminate \(request.bundleId) wasRunning=\(wasRunning), took \(duration)")
        return .object(["terminated": .bool(wasRunning)])
    }
}

