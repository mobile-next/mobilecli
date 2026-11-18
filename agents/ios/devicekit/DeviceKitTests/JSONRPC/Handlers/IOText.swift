// Portions of this file are derived from Maestro
// (https://github.com/mobile-dev-inc/maestro), licensed under the Apache License 2.0,
// and have been modified by Mobile Next HQ, Inc. See THIRD_PARTY_NOTICES.md.

import XCTest
import os

private enum Constants {
    static let typingFrequency = 30
    static let slowInputCharactersCount = 1
}

struct IOTextRequest: Codable {
    let text: String
}

@MainActor
struct IOTextMethodHandler: RPCMethodHandler {
    static let methodName = "device.io.text"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(IOTextRequest.self, from: params)

        do {
            let start = Date()

            await waitUntilKeyboardIsPresented()

            try await inputText(request.text)

            let duration = Date().timeIntervalSince(start)
            logger.info("Text input duration took \(duration)")
            return .object(["success": .bool(true)])
        } catch {
            logger.error("Error inputting text: \(error)")
            throw RPCMethodError.internalError("Error inputting text: \(error.localizedDescription)")
        }
    }

    private func waitUntilKeyboardIsPresented() async {
        try? await repeatUntil(timeout: 1, delta: 0.2) {
            let app = RunningApp.getForegroundApp() ?? XCUIApplication(bundleIdentifier: RunningApp.springboardBundleId)

            return app.keyboards.firstMatch.exists
        }
    }

    private func inputText(_ text: String) async throws {
        let firstCharacter = String(text.prefix(Constants.slowInputCharactersCount))
        logger.info("first character: \(firstCharacter)")
        var eventPath = PointerEventPath.pathForTextInput()
        eventPath.type(text: firstCharacter, typingSpeed: 1)
        let eventRecord = EventRecord(orientation: .portrait)
        _ = eventRecord.add(eventPath)
        try await RunnerDaemonProxy().synthesize(eventRecord: eventRecord)

        try await Task.sleep(nanoseconds: UInt64(1_000_000_000 * 0.5))

        if text.count > Constants.slowInputCharactersCount {
            let remainingText = String(text.suffix(text.count - Constants.slowInputCharactersCount))
            logger.info("remaining text: \(remainingText)")
            var eventPath2 = PointerEventPath.pathForTextInput()
            eventPath2.type(text: remainingText, typingSpeed: Constants.typingFrequency)
            let eventRecord2 = EventRecord(orientation: .portrait)
            _ = eventRecord2.add(eventPath2)
            try await RunnerDaemonProxy().synthesize(eventRecord: eventRecord2)
        }
    }

    private func repeatUntil(timeout: TimeInterval, delta: TimeInterval, block: () -> Bool) async throws {
        guard delta >= 0 else {
            throw NSError(domain: "Invalid value", code: 1, userInfo: [NSLocalizedDescriptionKey: "Delta cannot be negative"])
        }

        let deadline = Date().addingTimeInterval(timeout)

        while Date() < deadline {
            do {
                try await Task.sleep(nanoseconds: UInt64(1_000_000_000 * delta))
            } catch {
                throw NSError(domain: "Failed to sleep task", code: 2, userInfo: [NSLocalizedDescriptionKey: "Task could not be put to sleep"])
            }

            if block() {
                break
            }
        }
    }
}
