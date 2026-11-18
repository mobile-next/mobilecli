import Foundation
import XCTest

/// Runtime patches that keep XCTest out of the way of a long-lived automation runner.
/// Installed once when the test bundle loads, from the principal class (RunnerBackgrounding).
enum XCTestPatches {
    static func install() {
        skipQuiescenceWaits()
        keepRunningAfterIssues()
        showElementsBehindModalsIfRequested()
    }

    /// XCTest waits for the target app to go idle before every event. Apps with endless
    /// animations or video never do, so every event would stall until the idle timeout.
    private static func skipQuiescenceWaits() {
        let processClass: AnyClass? = NSClassFromString("XCUIApplicationProcess")
        let animationsIdle = NSSelectorFromString("waitForQuiescenceIncludingAnimationsIdle:")
        let animationsIdlePreEvent = NSSelectorFromString("waitForQuiescenceIncludingAnimationsIdle:isPreEvent:")

        if let method = class_getInstanceMethod(processClass, animationsIdle) {
            let skip: @convention(block) (AnyObject, Bool) -> Void = { _, _ in }
            method_setImplementation(method, imp_implementationWithBlock(skip))
        } else if let method = class_getInstanceMethod(processClass, animationsIdlePreEvent) {
            let skip: @convention(block) (AnyObject, Bool, Bool) -> Void = { _, _, _ in }
            method_setImplementation(method, imp_implementationWithBlock(skip))
        } else {
            NSLog("[DeviceKit] XCUIApplicationProcess has no quiescence method to skip")
        }
    }

    /// Internal XCTest failures (e.g. testmanagerd being slow on startup) would otherwise
    /// interrupt the single never-ending test that hosts the server.
    private static func keepRunningAfterIssues() {
        let shouldInterruptTest = NSSelectorFromString("shouldInterruptTest")
        guard let method = class_getInstanceMethod(NSClassFromString("XCTIssue"), shouldInterruptTest) else {
            NSLog("[DeviceKit] XCTIssue has no shouldInterruptTest to patch")
            return
        }

        let never: @convention(block) (AnyObject) -> Bool = { _ in false }
        method_setImplementation(method, imp_implementationWithBlock(never))
    }

    /// Launching with snapshotKeyHonorModalViews=false makes elements behind modals and
    /// dialogs show up in accessibility snapshots.
    private static func showElementsBehindModalsIfRequested() {
        guard ProcessInfo.processInfo.environment["snapshotKeyHonorModalViews"] == "false" else {
            return
        }

        NSLog("[DeviceKit] Disabling snapshotKeyHonorModalViews to show elements behind modals")
        AXClientSwizzler.overwriteDefaultParameters["snapshotKeyHonorModalViews"] = 0
    }
}
