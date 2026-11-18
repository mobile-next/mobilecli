import os

@MainActor
struct IOKeyboardStatusMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.keyboard.status"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        guard let foregroundApp = RunningApp.getForegroundApp() else {
            logger.warning("No foreground app found")
            throw RPCMethodError.internalError("No foreground app found")
        }

        let visible = foregroundApp.keyboards.firstMatch.exists
        return .object(["visible": .bool(visible)])
    }
}
