import Foundation
import XCTest

/// Remembers which applications XCTest cannot snapshot as a whole.
///
/// Snapshotting the application element of a heavy accessibility hierarchy — a deeply
/// nested React Native screen, for example — fails with kAXErrorIllegalArgument. The dump
/// recovers by starting at a window element instead, but the failed attempt is not free:
/// XCTest walks the hierarchy before giving up, which on a heavy screen is most of the
/// time a dump takes, and on a real device every step of that walk crosses USB.
///
/// Whether the whole-application snapshot works is a property of the running process, so
/// the verdict is remembered per process and later dumps go straight to the recovery path.
/// A relaunched app gets a new process id and is tried whole again, as is an app whose
/// process id could not be read when the failure was recorded.
@MainActor
enum WholeAppSnapshotSupport {
    private static var failedProcessIDs: [String: pid_t] = [:]

    /// Reports whether the whole-application snapshot of `application` already failed for
    /// the process that is running now.
    ///
    /// A stale verdict is the only way this can be wrong, so anything that suggests the
    /// process is not the one the failure was recorded for drops the entry and answers
    /// false: a different process id means the app was relaunched or reinstalled, and a
    /// recorded id of zero means the id was unreadable and the verdict can never be
    /// invalidated. Both cost one slow dump and recover on their own.
    static func isKnownToFail(_ application: XCUIApplication) -> Bool {
        guard let failedProcessID = failedProcessIDs[application.bundleID] else {
            return false
        }

        let processID = application.processID
        if failedProcessID == 0 || (processID != 0 && processID != failedProcessID) {
            failedProcessIDs.removeValue(forKey: application.bundleID)
            return false
        }

        return true
    }

    /// Records that the whole-application snapshot of `application` failed, so later dumps
    /// of the same process skip the attempt and start at the recovery element.
    static func recordFailure(_ application: XCUIApplication) {
        failedProcessIDs[application.bundleID] = application.processID
    }
}
