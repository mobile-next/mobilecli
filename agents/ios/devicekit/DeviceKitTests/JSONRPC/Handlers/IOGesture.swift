import Foundation
import os

struct Action: Codable {
    let type: String
    let duration: TimeInterval
    let x: Float
    let y: Float
    let button: Int
    /// Seconds after the gesture starts that this finger touches down. Only valid on a finger's
    /// press; without it every finger touches down at once. Lets a gesture tap more than once:
    /// VoiceOver's double tap is two fingers on the same spot, the second starting after the first lifts.
    let start: TimeInterval?
}

struct IOGestureRequest: Codable {
    let actions: [Action]
}

private enum ActionType: String {
    case press
    case move
    case release
}

@MainActor
struct IOGestureMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.gesture"

    private static let minimumPressHoldDuration: TimeInterval = 0.05

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOGestureRequest.self, from: params)

        guard !request.actions.isEmpty else {
            throw RPCMethodError.invalidParams("Actions array cannot be empty")
        }

        let fingerActions = groupActionsByFinger(request.actions)
        logger.info("Gesture has \(fingerActions.count) finger(s)")

        for (fingerIndex, actions) in fingerActions {
            try validateFingerSequence(actions, fingerIndex: fingerIndex)
            try validateStart(actions, fingerIndex: fingerIndex)
        }

        do {
            let start = Date()
            try await executeGesture(fingerActions: fingerActions)
            let duration = Date().timeIntervalSince(start)
            logger.info("Gesture execution completed in \(duration)s")
            return .object(["success": .bool(true)])
        } catch let error as RPCMethodError {
            throw error
        } catch {
            logger.error("Error executing gesture: \(error)")
            throw RPCMethodError.internalError("Gesture execution failed: \(error.localizedDescription)")
        }
    }

    private func groupActionsByFinger(_ actions: [Action]) -> [Int: [Action]] {
        var grouped: [Int: [Action]] = [:]
        for action in actions {
            grouped[action.button, default: []].append(action)
        }
        return grouped
    }

    private func validateStart(_ actions: [Action], fingerIndex: Int) throws {
        for (index, action) in actions.enumerated() {
            guard let start = action.start else { continue }
            guard index == 0 else {
                throw RPCMethodError.invalidParams(
                    "Finger \(fingerIndex) has 'start' at index \(index), only its press can have one"
                )
            }
            guard start >= 0 else {
                throw RPCMethodError.invalidParams("Negative start \(start) for finger \(fingerIndex)")
            }
        }
    }

    private func validateFingerSequence(_ actions: [Action], fingerIndex: Int) throws {
        guard !actions.isEmpty else {
            throw RPCMethodError.invalidParams("Finger \(fingerIndex) has no actions")
        }

        guard actions.first?.type == ActionType.press.rawValue else {
            throw RPCMethodError.invalidParams(
                "Finger \(fingerIndex) must start with 'press' action, got '\(actions.first?.type ?? "nil")'"
            )
        }

        guard actions.last?.type == ActionType.release.rawValue else {
            throw RPCMethodError.invalidParams(
                "Finger \(fingerIndex) must end with 'release' action, got '\(actions.last?.type ?? "nil")'"
            )
        }

        var hasPressed = false
        var hasReleased = false

        for (index, action) in actions.enumerated() {
            guard let actionType = ActionType(rawValue: action.type) else {
                throw RPCMethodError.invalidParams(
                    "Unknown action type '\(action.type)' for finger \(fingerIndex) at index \(index)"
                )
            }

            guard action.x >= 0 && action.y >= 0 else {
                throw RPCMethodError.invalidParams(
                    "Negative coordinates (\(action.x), \(action.y)) for finger \(fingerIndex) at index \(index)"
                )
            }

            guard action.duration >= 0 else {
                throw RPCMethodError.invalidParams(
                    "Negative duration \(action.duration) for finger \(fingerIndex) at index \(index)"
                )
            }

            switch actionType {
            case .press:
                if hasPressed {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has multiple 'press' actions"
                    )
                }
                hasPressed = true

            case .move:
                if !hasPressed {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has 'move' before 'press' at index \(index)"
                    )
                }
                if hasReleased {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has 'move' after 'release' at index \(index)"
                    )
                }

            case .release:
                if !hasPressed {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has 'release' before 'press' at index \(index)"
                    )
                }
                if hasReleased {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has multiple 'release' actions"
                    )
                }
                if index != actions.count - 1 {
                    throw RPCMethodError.invalidParams(
                        "Finger \(fingerIndex) has 'release' before end of sequence at index \(index)"
                    )
                }
                hasReleased = true
            }
        }
    }

    private func executeGesture(fingerActions: [Int: [Action]]) async throws {
        let isMultiFinger = fingerActions.count > 1
        let style: EventRecord.Style = isMultiFinger ? .multiFinger : .singleFinger

        let geometry = OrientationGeometry.current()
        let eventRecord = EventRecord(orientation: .portrait, style: style, displayID: geometry.displayID)

        for (fingerIndex, actions) in fingerActions.sorted(by: { $0.key < $1.key }) {
            logger.info("Building path for finger \(fingerIndex) with \(actions.count) actions")
            try buildFingerPath(
                actions: actions,
                fingerIndex: fingerIndex,
                geometry: geometry,
                eventRecord: eventRecord
            )
        }

        logger.info("Synthesizing gesture with \(fingerActions.count) finger path(s)")
        try await RunnerDaemonProxy().synthesize(eventRecord: eventRecord)
    }

    private func buildFingerPath(
        actions: [Action],
        fingerIndex: Int,
        geometry: OrientationGeometry,
        eventRecord: EventRecord
    ) throws {
        guard let pressAction = actions.first else {
            throw RPCMethodError.invalidParams("Finger \(fingerIndex) has no actions")
        }

        let initialPoint = geometry.touchPoint(for: CGPoint(x: CGFloat(pressAction.x), y: CGFloat(pressAction.y)))

        var currentOffset: TimeInterval = pressAction.start ?? 0

        var path = PointerEventPath.pathForTouch(at: initialPoint, offset: currentOffset)

        currentOffset += max(pressAction.duration, Self.minimumPressHoldDuration)

        for action in actions.dropFirst() {
            let point = geometry.touchPoint(for: CGPoint(x: CGFloat(action.x), y: CGFloat(action.y)))

            guard let actionType = ActionType(rawValue: action.type) else {
                continue
            }

            switch actionType {
            case .press:
                break

            case .move:
                currentOffset += action.duration
                path.offset = currentOffset
                path.moveTo(point: point)
                logger.debug("Finger \(fingerIndex): move to (\(point.x), \(point.y)) at offset \(currentOffset)s")

            case .release:
                currentOffset += action.duration
                path.offset = currentOffset
                path.liftUp()
                logger.debug("Finger \(fingerIndex): release at offset \(currentOffset)s")
            }
        }

        _ = eventRecord.add(path)
        logger.info("Finger \(fingerIndex): path completed, total duration \(currentOffset)s")
    }
}
