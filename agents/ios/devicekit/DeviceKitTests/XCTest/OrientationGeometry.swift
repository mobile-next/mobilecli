import XCTest

struct AppWindow {
    // 0 when the window is on the main display
    let displayID: UInt64
    let size: CGSize
}

// maps screen points, as seen in screenshots, onto the touch space of the
// display the foreground app is on
struct OrientationGeometry {
    let portraitWidth: CGFloat
    let portraitHeight: CGFloat
    let orientation: UIDeviceOrientation
    // nil when the app is on the main display
    let displayID: UInt64?

    private static let springboardBundleId = "com.apple.springboard"

    static func current() -> OrientationGeometry {
        let app = RunningApp.getForegroundApp()
            ?? XCUIApplication(bundleIdentifier: springboardBundleId)

        var size = app.frame.size
        if size.width <= 0 || size.height <= 0 {
            NSLog("Foreground app has no frame, falling back to SpringBoard.")
            size = XCUIApplication(bundleIdentifier: springboardBundleId).frame.size
        }

        let windows = isMultiDisplay() ? appWindows(of: app) : []
        let displayID = secondaryDisplayID(of: app, windows: windows)

        // springboard reports the cover display's frame even when unfolded
        if let displayID {
            let displaySizes = windows.filter { $0.displayID == displayID }.map(\.size)
            if !displaySizes.contains(size), let displaySize = mostCommon(displaySizes) {
                size = displaySize
            }
        }

        return OrientationGeometry(
            portraitWidth: min(size.width, size.height),
            portraitHeight: max(size.width, size.height),
            orientation: touchOrientation(of: app),
            displayID: displayID
        )
    }

    func touchPoint(for point: CGPoint) -> CGPoint {
        switch orientation {
        case .landscapeLeft:
            return CGPoint(x: portraitWidth - point.y, y: point.x)

        case .landscapeRight:
            return CGPoint(x: point.y, y: portraitHeight - point.x)

        case .portraitUpsideDown:
            return CGPoint(x: portraitWidth - point.x, y: portraitHeight - point.y)

        default:
            return point
        }
    }

    // interface orientation of the foreground app, which taps and the ui tree
    // follow, expressed as the UIDeviceOrientation touchPoint is written for
    static func interfaceOrientation() -> UIDeviceOrientation {
        let app = RunningApp.getForegroundApp()
            ?? XCUIApplication(bundleIdentifier: springboardBundleId)
        return touchOrientation(of: app)
    }

    // springboard lays its elements out, status bar included, on the panel's
    // portrait axes however the interface is turned, even when its own frame
    // is landscape. nil when the interface is upright and there is nothing to undo
    static func springboardToInterface() -> OrientationGeometry? {
        let orientation = interfaceOrientation()
        let rotations: [UIDeviceOrientation] = [.landscapeLeft, .landscapeRight, .portraitUpsideDown]
        guard rotations.contains(orientation) else {
            return nil
        }

        let size = XCUIApplication(bundleIdentifier: springboardBundleId).frame.size
        guard size.width > 0, size.height > 0 else {
            return nil
        }
        return OrientationGeometry(
            portraitWidth: min(size.width, size.height),
            portraitHeight: max(size.width, size.height),
            orientation: orientation,
            displayID: nil
        )
    }

    // inverse of touchPoint, for a whole frame
    func interfaceFrame(for panelFrame: AXFrame) -> AXFrame {
        let x = panelFrame["X"] ?? 0
        let y = panelFrame["Y"] ?? 0
        let width = panelFrame["Width"] ?? 0
        let height = panelFrame["Height"] ?? 0
        let panelWidth = Double(portraitWidth)
        let panelHeight = Double(portraitHeight)

        switch orientation {
        case .landscapeLeft:
            return ["X": y, "Y": panelWidth - x - width, "Width": height, "Height": width]

        case .landscapeRight:
            return ["X": panelHeight - y - height, "Y": x, "Width": height, "Height": width]

        case .portraitUpsideDown:
            return ["X": panelWidth - x - width, "Y": panelHeight - y - height, "Width": width, "Height": height]

        default:
            return panelFrame
        }
    }

    // touches follow the app's interface orientation, not the device's: the
    // unfolded iPhone Duo draws its app in landscape while the device reports
    // portrait. UIInterfaceOrientation.landscapeRight shares its raw value with
    // UIDeviceOrientation.landscapeLeft (and vice versa), so the raw value maps
    // straight onto the device orientation touchPoint is written for
    private static func touchOrientation(of app: XCUIApplication) -> UIDeviceOrientation {
        let selector = NSSelectorFromString("interfaceOrientation")
        if app.responds(to: selector),
           let rawValue = app.value(forKey: "interfaceOrientation") as? Int,
           let orientation = UIDeviceOrientation(rawValue: rawValue),
           orientation != .unknown {
            return orientation
        }

        let orientation = XCUIDevice.shared.orientation
        if orientation == .unknown {
            return .portrait
        }
        return orientation
    }

    // foldables show the unfolded app on a secondary display; events without
    // a display id go to the main one and never reach the app. single-display
    // devices skip the snapshot this needs
    static func secondaryDisplayID(of app: XCUIApplication) -> UInt64? {
        guard isMultiDisplay() else {
            return nil
        }
        return secondaryDisplayID(of: app, windows: appWindows(of: app))
    }

    private static func secondaryDisplayID(of app: XCUIApplication, windows: [AppWindow]) -> UInt64? {
        let mainDisplayID = displayID(of: XCUIScreen.main)
        let isOnMainDisplay = windows.contains { $0.displayID == 0 || $0.displayID == mainDisplayID }

        // springboard keeps windows on both displays of a foldable, lit or
        // not; it draws in landscape only on the unfolded inner display
        if isOnMainDisplay, !touchOrientation(of: app).isLandscape {
            return nil
        }
        return windows.first { $0.displayID != 0 && $0.displayID != mainDisplayID }?.displayID
    }

    // portrait size of the unfolded inner display, nil when the app is on the
    // main display. single-display devices skip the foreground app lookup
    static func secondaryDisplaySize() -> CGSize? {
        guard isMultiDisplay() else {
            return nil
        }

        let geometry = current()
        guard geometry.displayID != nil else {
            return nil
        }
        return CGSize(width: geometry.portraitWidth, height: geometry.portraitHeight)
    }

    private static func isMultiDisplay() -> Bool {
        XCUIScreen.screens.count > 1
    }

    private static func displayID(of screen: XCUIScreen) -> UInt64? {
        guard screen.responds(to: NSSelectorFromString("displayID")) else {
            return nil
        }
        return (screen.value(forKey: "displayID") as? NSNumber)?.uint64Value
    }

    private static func mostCommon(_ sizes: [CGSize]) -> CGSize? {
        let counts = Dictionary(grouping: sizes, by: { "\($0.width)x\($0.height)" })
        return counts.values.max { $0.count < $1.count }?.first
    }

    // the application element reports display 0; its windows carry the real
    // display id, so a snapshot two levels deep is enough
    private static func appWindows(of app: XCUIApplication) -> [AppWindow] {
        let previousMaxDepth = AXClientSwizzler.overwriteDefaultParameters["maxDepth"]
        AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = 2
        defer {
            AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = previousMaxDepth
        }

        guard let root = try? app.snapshot().dictionaryRepresentation else {
            return []
        }

        let displayKey = XCUIElement.AttributeName(rawValue: "displayID")
        let frameKey = XCUIElement.AttributeName(rawValue: "frame")
        let childrenKey = XCUIElement.AttributeName(rawValue: "children")
        let windows = root[childrenKey] as? [[XCUIElement.AttributeName: Any]] ?? []
        return windows.compactMap { window in
            guard let displayID = window[displayKey] as? Int,
                  let frame = window[frameKey] as? AXFrame,
                  let width = frame["Width"], let height = frame["Height"] else {
                return nil
            }
            return AppWindow(displayID: UInt64(displayID), size: CGSize(width: width, height: height))
        }
    }
}
