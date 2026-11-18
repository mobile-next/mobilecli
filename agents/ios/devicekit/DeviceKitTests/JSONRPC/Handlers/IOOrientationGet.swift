import os

@MainActor
struct IOOrientationGetMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.orientation.get"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let start = Date()
        // the app's orientation, not the device's: a device lying flat is
        // faceUp, and a landscape-only app stays landscape on a portrait device.
        // screenshots, device.info, taps and the ui tree all follow the app
        let value = OrientationGeometry.interfaceOrientation().isLandscape ? "LANDSCAPE" : "PORTRAIT"

        let duration = Date().timeIntervalSince(start)
        logger.info("Get orientation took \(duration), result: \(value)")
        return .object(["orientation": .string(value)])
    }
}
