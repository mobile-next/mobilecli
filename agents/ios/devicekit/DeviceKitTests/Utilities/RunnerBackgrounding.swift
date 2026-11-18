import UIKit

/// Principal class of the test bundle (NSPrincipalClass): XCTest instantiates it when the bundle
/// loads, before UI testing initializes, which also makes it the place to install XCTestPatches.
///
/// XCUIInitializeForUITesting waits up to 30s for the test runner to enter the background.
/// xcodebuild's testmanagerd session does that, but on iOS 27 simulators nothing backgrounds a
/// runner started with a plain `simctl launch`, so initialization fails with
/// "Failed to background test runner within 30.0s". Suspend ourselves in that case.
@objc(RunnerBackgrounding)
final class RunnerBackgrounding: NSObject {
    private static let suspendDelay: TimeInterval = 2.0

    override init() {
        super.init()
        XCTestPatches.install()
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.suspendDelay) {
            Self.suspendIfInForeground()
        }
    }

    private static func suspendIfInForeground() {
        let application = UIApplication.shared
        if application.applicationState == .background {
            return
        }

        let suspend = NSSelectorFromString("suspend")
        guard application.responds(to: suspend) else {
            NSLog("[DeviceKit] UIApplication does not respond to -suspend, cannot background the test runner")
            return
        }

        NSLog("[DeviceKit] Suspending test runner so UI testing can initialize")
        application.perform(suspend)
    }
}
