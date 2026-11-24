// Portions of this file are derived from Maestro
// (https://github.com/mobile-dev-inc/maestro), licensed under the Apache License 2.0,
// and have been modified by Mobile Next HQ, Inc. See THIRD_PARTY_NOTICES.md.

import Foundation
import XCTest
import os

struct RunningApp {
    static let springboardBundleId = "com.apple.springboard"

    private static let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    private init() {}

    static func getForegroundApp() -> XCUIApplication? {
        let runningAppIds = XCUIApplication.activeAppsInfo().compactMap {
            $0["bundleId"] as? String
        }

        NSLog("Detected running apps: \(runningAppIds)")

        // the only listed app is not always in front (Safari in the background
        // while an app under Rosetta is in front), so check its state too
        return runningAppIds
            .map { XCUIApplication(bundleIdentifier: $0) }
            .first { $0.state == .runningForeground }
    }

}
