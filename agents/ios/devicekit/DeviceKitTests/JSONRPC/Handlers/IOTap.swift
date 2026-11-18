import os

struct IOTapRequest: Codable {
    let x: Float
    let y: Float
}

@MainActor
struct IOTapMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.tap"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOTapRequest.self, from: params)

        let geometry = OrientationGeometry.current()
        let point = geometry.touchPoint(for: CGPoint(x: CGFloat(request.x), y: CGFloat(request.y)))

        do {
            let eventRecord = EventRecord(orientation: .portrait, displayID: geometry.displayID)
            _ = eventRecord.addPointerTouchEvent(
                at: point,
                touchUpAfter: nil
            )
            let start = Date()
            try await RunnerDaemonProxy().synthesize(eventRecord: eventRecord)
            let duration = Date().timeIntervalSince(start)
            logger.info("Tapping took \(duration)")
            return .object(["success": .bool(true)])
        } catch {
            logger.error("Error tapping: \(error)")
            throw RPCMethodError.internalError("Error tapping point: \(error.localizedDescription)")
        }
    }
}

