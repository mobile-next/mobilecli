import os

private enum Constants {
    static let defaultSwipeDuration = 0.1
    static let maxSwipeDuration = 60.0
}

struct IOSwipeRequest: Decodable {
    let x1: Int
    let y1: Int
    let x2: Int
    let y2: Int
    let duration: TimeInterval?
}

@MainActor
struct IOSwipeMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.swipe"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOSwipeRequest.self, from: params)

        let duration = request.duration ?? Constants.defaultSwipeDuration
        guard duration >= 0 else {
            throw RPCMethodError.invalidParams("Duration cannot be negative, got \(duration)")
        }
        guard duration <= Constants.maxSwipeDuration else {
            throw RPCMethodError.invalidParams(
                "Duration \(duration) exceeds the maximum of \(Constants.maxSwipeDuration) seconds"
            )
        }

        let geometry = OrientationGeometry.current()
        let start = geometry.touchPoint(for: CGPoint(x: request.x1, y: request.y1))
        let end = geometry.touchPoint(for: CGPoint(x: request.x2, y: request.y2))

        do {
            try await swipePrivateAPI(
                start: start,
                end: end,
                duration: duration,
                displayID: geometry.displayID
            )

            return .object(["success": .bool(true)])
        } catch {
            logger.error("Error performing swipe: \(error)")
            throw RPCMethodError.internalError("Error performing swipe: \(error.localizedDescription)")
        }
    }

    func swipePrivateAPI(start: CGPoint, end: CGPoint, duration: Double, displayID: UInt64?) async throws {
        logger.info("Swipe (v1) from \(start.debugDescription) to \(end.debugDescription) with duration \(duration)")

        let eventRecord = EventRecord(orientation: .portrait, displayID: displayID)
        _ = eventRecord.addSwipeEvent(start: start, end: end, duration: duration)

        try await RunnerDaemonProxy().synthesize(eventRecord: eventRecord)
    }
}
