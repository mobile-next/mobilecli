// Portions of this file are derived from Maestro
// (https://github.com/mobile-dev-inc/maestro), licensed under the Apache License 2.0,
// and have been modified by Mobile Next HQ, Inc. See THIRD_PARTY_NOTICES.md.

import FlyingFox
import XCTest
import os

/// UI tests for the DeviceKit iOS application.
///
final class DeviceKitUITests: XCTestCase {

    private static let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: "DeviceKitUITests"
    )

    override func setUpWithError() throws {
        continueAfterFailure = true

        // prevent the runner from halting
        // when testmanagerd is slow to connect on startup (random ~20s timeout).
        // shouldHaltWhenReceivesControl is a private XCTestCase BOOL property.
        let sel = NSSelectorFromString("setShouldHaltWhenReceivesControl:")
        if responds(to: sel) {
            setValue(NSNumber(value: false), forKey: "shouldHaltWhenReceivesControl")
        }

        // prevent XCTest from resetting shouldHaltWhenReceivesControl back to YES
        let sel2 = NSSelectorFromString("setShouldSetShouldHaltWhenReceivesControl:")
        if responds(to: sel2) {
            setValue(NSNumber(value: false), forKey: "shouldSetShouldHaltWhenReceivesControl")
        }
    }

    // swallow XCTest issues instead of propagating them up. XCTestPatches makes
    // shouldInterruptTest return NO; this override keeps issues away from XCTest's
    // failure machinery.
    override func record(_ issue: XCTIssue) {
        Self.logger.warning("XCTest issue (swallowed): \(issue.compactDescription)")
    }

    override class func setUp() {
        logger.trace("setUp")
    }

    @MainActor
    func testRunAutomation() async throws {
        let server = XCTestServer()
        DeviceKitUITests.logger.info("Will start WebSocket JSON-RPC server")
        do {
            try await server.start()
        } catch {
            // record(_:) above swallows every issue; go straight to super so a server that
            // never started fails the test run instead of reporting it as passed.
            DeviceKitUITests.logger.error("Server failed: \(error)")
            super.record(XCTIssue(type: .thrownError, compactDescription: "Server failed: \(error)"))
        }
    }

    override class func tearDown() {
        logger.trace("tearDown")
    }
}
