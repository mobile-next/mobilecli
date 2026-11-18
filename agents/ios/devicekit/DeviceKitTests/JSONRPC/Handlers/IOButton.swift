import os

struct IOButtonRequest : Codable {
    enum Button: String, Codable {
        case home
        case lock
        case volumeUp
        case volumeDown
    }

    let button: Button
}

@MainActor
struct IOButtonMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.button"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOButtonRequest.self, from: params)

        let start = Date()

        logger.info("[Start] Tapping on button: \(request.button.rawValue)")
        switch request.button {
        case .home:
            XCUIDevice.shared.press(.home)
        case .lock:
            XCUIDevice.shared.perform(NSSelectorFromString("pressLockButton"))
        case .volumeUp:
            #if targetEnvironment(simulator)
            logger.warning("volumeUp button is not available on the Simulator")
            #else
            XCUIDevice.shared.press(.volumeUp)
            #endif
        case .volumeDown:
            #if targetEnvironment(simulator)
            logger.warning("volumeDown button is not available on the Simulator")
            #else
            XCUIDevice.shared.press(.volumeDown)
            #endif
        }
        logger.info("[Done] Tapping on button: \(request.button.rawValue)")

        let duration = Date().timeIntervalSince(start)
        logger.info("Button Tap duration took \(duration)")
        return .object(["success": .bool(true)])
    }
}

