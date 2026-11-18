// Portions of this file are derived from Maestro
// (https://github.com/mobile-dev-inc/maestro), licensed under the Apache License 2.0,
// and have been modified by Mobile Next HQ, Inc. See THIRD_PARTY_NOTICES.md.

import os

extension Logger {
    func measure<T>(message: String, _ block: () throws -> T) rethrows -> T {
        let start = Date()
        info("\(message) - start")

        let result = try block()

        let duration = Date().timeIntervalSince(start)
        NSLog("\(message) - duration \(duration)")

        return result
    }
}

struct DumpUIRequest: Codable {
    let format: String?
}

private enum DumpUIFormat: String {
    case json
    case raw

    init(from string: String?) {
        if let string = string?.lowercased(), let format = DumpUIFormat(rawValue: string) {
            self = format
        } else {
            self = .json
        }
    }
}

@MainActor
struct DumpUIMethodHandler: RPCMethodHandler {
    static let methodName = "device.dump.ui"

    private let springboardApplication = XCUIApplication(
        bundleIdentifier: "com.apple.springboard"
    )
    private let snapshotMaxDepth = 60

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(DumpUIRequest.self, from: params)

        let format = DumpUIFormat(from: request.format)
        logger.info("dump_ui requested with format: \(format.rawValue)")

        do {
            let foregroundApp = RunningApp.getForegroundApp()
            guard let foregroundApp = foregroundApp else {
                logger.warning("No foreground app found, returning springboard app hierarchy")
                let springboardHierarchy = springboardAppInInterfaceSpace(
                    try elementHierarchy(xcuiElement: springboardApplication),
                    OrientationGeometry.springboardToInterface()
                )
                return try formatResponse(axElement: springboardHierarchy, format: format)
            }

            logger.info("[Start] View hierarchy snapshot for \(foregroundApp)")
            let appViewHierarchy = try logger.measure(
                message: "View hierarchy snapshot for \(foregroundApp)"
            ) {
                try getAppViewHierarchy(
                    foregroundApp: foregroundApp,
                    excludeKeyboardElements: false
                )
            }

            logger.info("[Done] View hierarchy snapshot for \(foregroundApp)")
            return try formatResponse(axElement: appViewHierarchy, format: format)
        } catch let error as RPCMethodError {
            throw error
        } catch let error as NSError {
            if error.domain == "com.apple.dt.XCTest.XCTFuture",
               error.code == 1000,
               error.localizedDescription.contains("Timed out while evaluating UI query") {
                throw RPCMethodError.timeout(error.localizedDescription)
            } else if error.domain == "com.apple.dt.xctest.automation-support.error",
                      error.code == 6,
                      error.localizedDescription.contains("Unable to perform work on main run loop") {
                throw RPCMethodError.timeout(error.localizedDescription)
            } else {
                throw RPCMethodError.internalError("Snapshot failure: \(error.localizedDescription)")
            }
        }
    }

    private func formatResponse(axElement: AXElement, format: DumpUIFormat) throws -> JSONValue {
        switch format {
        case .json:
            let sourceTree = SourceTreeElement(axElement: axElement)
            return try JSONValue.from(sourceTree)

        case .raw:
            return try JSONValue.from(axElement)
        }
    }

    private func getAppViewHierarchy(
        foregroundApp: XCUIApplication,
        excludeKeyboardElements: Bool
    ) throws -> AXElement {
        SystemPermissionManager.handleSystemPermissionAlertIfNeeded(foregroundApp: foregroundApp)
        var appHierarchy = try getHierarchyWithFallback(foregroundApp)

        // springboard lives on the main display; an app on another display
        // (an unfolded foldable) has nothing to line up with it
        let isOnSecondaryDisplay = OrientationGeometry.secondaryDisplayID(of: foregroundApp) != nil
        let springboardToInterface = isOnSecondaryDisplay ? nil : OrientationGeometry.springboardToInterface()

        if foregroundApp.bundleID == "com.apple.springboard" {
            appHierarchy = springboardAppInInterfaceSpace(appHierarchy, springboardToInterface)
        }

        let statusBars = (logger.measure(message: "Fetch status bar hierarchy") {
            fullStatusBars(springboardApplication)
        } ?? []).map { inInterfaceSpace($0, springboardToInterface) }

        let safariWebViewHierarchy = logger.measure(message: "Fetch Safari WebView hierarchy") {
            getSafariWebViewHierarchy()
        }

        let deviceFrame = springboardApplication.frame
        let deviceAxFrame: AXFrame = [
            "X": Double(deviceFrame.minX),
            "Y": Double(deviceFrame.minY),
            "Width": Double(deviceFrame.width),
            "Height": Double(deviceFrame.height),
        ]
        let appFrame = appHierarchy.frame

        // in landscape springboard stays portrait while the app is rotated;
        // the app frames are already in screen coordinates, don't shift them
        let isRotatedDeviceFrame = deviceAxFrame["Width"] == appFrame["Height"] && deviceAxFrame["Height"] == appFrame["Width"]

        if deviceAxFrame != appFrame, !isRotatedDeviceFrame, !isOnSecondaryDisplay,
           let deviceWidth = deviceAxFrame["Width"], deviceWidth > 0,
           let deviceHeight = deviceAxFrame["Height"], deviceHeight > 0,
           let appWidth = appFrame["Width"], appWidth > 0,
           let appHeight = appFrame["Height"], appHeight > 0 {
            let offset = WindowOffset(offsetX: deviceWidth - appWidth, offsetY: deviceHeight - appHeight)
            logger.info("Adjusting view hierarchy with offset: \(String.init(describing: offset))")
            return buildRootElement(app: expandElementSizes(appHierarchy, offset: offset), statusBars: statusBars, safari: safariWebViewHierarchy)
        }

        return buildRootElement(app: appHierarchy, statusBars: statusBars, safari: safariWebViewHierarchy)
    }

    // springboard's application element already reports the rotated screen;
    // only the elements inside it are on the panel's axes
    private func springboardAppInInterfaceSpace(_ app: AXElement, _ geometry: OrientationGeometry?) -> AXElement {
        var mapped = app
        mapped.children = app.children?.map { inInterfaceSpace($0, geometry) }
        return mapped
    }

    private func inInterfaceSpace(_ springboardElement: AXElement, _ geometry: OrientationGeometry?) -> AXElement {
        guard let geometry else {
            return springboardElement
        }
        return springboardElement.mappingFrames(geometry.interfaceFrame(for:))
    }

    private func buildRootElement(app: AXElement, statusBars: [AXElement], safari: AXElement?) -> AXElement {
        AXElement(children: [app, AXElement(children: statusBars), safari].compactMap { $0 })
    }

    private func expandElementSizes(_ element: AXElement, offset: WindowOffset) -> AXElement {
        let adjustedFrame: AXFrame = [
            "X": (element.frame["X"] ?? 0) + offset.offsetX,
            "Y": (element.frame["Y"] ?? 0) + offset.offsetY,
            "Width": element.frame["Width"] ?? 0,
            "Height": element.frame["Height"] ?? 0,
        ]
        let adjustedChildren = element.children?.map { expandElementSizes($0, offset: offset) } ?? []

        return AXElement(
            identifier: element.identifier,
            frame: adjustedFrame,
            value: element.value,
            title: element.title,
            label: element.label,
            elementType: element.elementType,
            enabled: element.enabled,
            horizontalSizeClass: element.horizontalSizeClass,
            verticalSizeClass: element.verticalSizeClass,
            placeholderValue: element.placeholderValue,
            selected: element.selected,
            hasFocus: element.hasFocus,
            displayID: element.displayID,
            windowContextID: element.windowContextID,
            children: adjustedChildren
        )
    }

    /// Snapshots `element` and everything below it, falling back when XCTest refuses.
    ///
    /// Three things can happen. The snapshot succeeds and is returned. It succeeds but
    /// reaches `snapshotMaxDepth`, and the children are re-snapshotted one by one so the
    /// tree is not silently cut short. Or XCTest rejects it with kAXErrorIllegalArgument,
    /// which a hierarchy too large to snapshot in one piece does, and the tree is built
    /// from a window element instead — a failure that is remembered for the application,
    /// since repeating an attempt that walks the whole hierarchy before failing is what
    /// makes a dump of a heavy screen slow.
    private func getHierarchyWithFallback(_ element: XCUIElement) throws -> AXElement {
        logger.info("Starting getHierarchyWithFallback for element.")

        if let application = element as? XCUIApplication,
           WholeAppSnapshotSupport.isKnownToFail(application) {
            logger.info(
                "Skipping the whole-application snapshot for \(application.bundleID): it failed for this process"
            )
            return try recoveredHierarchy(element)
        }

        let startedAt = Date()
        do {
            var hierarchy = try elementHierarchy(xcuiElement: element)
            logger.info("Successfully retrieved element hierarchy.")

            if hierarchy.depth() < snapshotMaxDepth {
                return hierarchy
            }
            logger.info("Hierarchy hit the depth limit of \(snapshotMaxDepth), snapshotting children one by one")
            let count = try directChildCount(of: element)
            var children: [AXElement] = []
            for i in 0..<count {
                let element = element.descendants(matching: .other).element(boundBy: i).firstMatch
                children.append(try getHierarchyWithFallback(element))
            }
            hierarchy.children = children
            return hierarchy
        } catch let error {
            guard isIllegalArgumentError(error) else {
                throw error
            }

            let elapsed = Date().timeIntervalSince(startedAt)
            logger.error("Whole-element snapshot failed after \(elapsed) seconds, recovering")
            if let application = element as? XCUIApplication {
                WholeAppSnapshotSupport.recordFailure(application)
            }

            return try recoveredHierarchy(element)
        }
    }

    /// Builds the hierarchy without snapshotting `element` as a whole: it starts at the
    /// deepest single-child descendant that still has something below it, and collects the
    /// keyboard, alerts and custom windows that are rooted outside that subtree separately.
    private func recoveredHierarchy(_ element: XCUIElement) throws -> AXElement {
        AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = snapshotMaxDepth

        let recoveryElement = try logger.measure(message: "Find recovery element") {
            try findRecoveryElement(element.children(matching: .any).firstMatch)
        }
        let hierarchy = try logger.measure(message: "Recovery element hierarchy") {
            try getHierarchyWithFallback(recoveryElement)
        }

        guard let element = element as? XCUIApplication else {
            return hierarchy
        }

        let keyboard = logger.measure(message: "Fetch keyboard hierarchy") {
            keyboardHierarchy(element)
        }

        let alerts = logger.measure(message: "Fetch alert hierarchy") {
            fullScreenAlertHierarchy(element)
        }

        let other = try logger.measure(message: "Fetch other custom element from window") {
            try customWindowElements(element)
        }
        return AXElement(children: [other, keyboard, alerts, hierarchy].compactMap { $0 })
    }

    /// Counts an element's direct children without snapshotting everything below them.
    ///
    /// `snapshot()` returns the entire subtree, so using it to count children costs as much
    /// as the dump itself, once per element asked. The callers here only need the number of
    /// children, and a two-level snapshot carries exactly that, the way OrientationGeometry
    /// reads window attributes without pulling in their contents.
    private func directChildCount(of element: XCUIElement) throws -> Int {
        let previousMaxDepth = AXClientSwizzler.overwriteDefaultParameters["maxDepth"]
        AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = 2
        defer {
            AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = previousMaxDepth
        }

        return try element.snapshot().children.count
    }

    /// Reports whether XCTest refused to snapshot an element because the hierarchy was
    /// not something it could return in one piece, which is the failure the recovery path
    /// exists for. Any other error belongs to the caller.
    private func isIllegalArgumentError(_ error: Error) -> Bool {
        error.localizedDescription.contains("Error kAXErrorIllegalArgument getting snapshot for element")
    }

    private func keyboardHierarchy(_ element: XCUIApplication) -> AXElement? {
        guard element.keyboards.firstMatch.exists else { return nil }
        let keyboard = element.keyboards.firstMatch
        return try? elementHierarchy(xcuiElement: keyboard)
    }

    private func customWindowElements(_ element: XCUIApplication) throws -> AXElement? {
        let windowElement = element.children(matching: .any).firstMatch
        if try directChildCount(of: windowElement) > 1 {
            return nil
        }
        return try? elementHierarchy(xcuiElement: windowElement)
    }

    private func fullScreenAlertHierarchy(_ element: XCUIApplication) -> AXElement? {
        guard element.alerts.firstMatch.exists else { return nil }
        let alert = element.alerts.firstMatch
        return try? elementHierarchy(xcuiElement: alert)
    }

    private func fullStatusBars(_ element: XCUIApplication) -> [AXElement]? {
        guard element.statusBars.firstMatch.exists else { return nil }
        return try? element.statusBars.allElementsBoundByIndex.compactMap { statusBar in
            try elementHierarchy(xcuiElement: statusBar)
        }
    }

    private func getSafariWebViewHierarchy() -> AXElement? {
        let systemVersion = ProcessInfo.processInfo.operatingSystemVersion
        guard systemVersion.majorVersion >= 26 else { return nil }

        let safariWebService = XCUIApplication(bundleIdentifier: "com.apple.SafariViewService")
        let isRunning = safariWebService.state == .runningForeground || safariWebService.state == .runningBackground
        guard isRunning else { return nil }

        let webViewCount = safariWebService.webViews.count
        guard webViewCount > 0 else { return nil }

        logger.info("[Start] Fetching Safari WebView hierarchy (\(webViewCount) webview(s) detected)")

        do {
            AXClientSwizzler.overwriteDefaultParameters["maxDepth"] = snapshotMaxDepth
            let safariHierarchy = try elementHierarchy(xcuiElement: safariWebService)
            logger.info("[Done] Safari WebView hierarchy fetched successfully")
            return safariHierarchy
        } catch {
            logger.error("[Error] Failed to fetch Safari WebView hierarchy: \(error.localizedDescription)")
            return nil
        }
    }

    private func findRecoveryElement(_ element: XCUIElement) throws -> XCUIElement {
        if try directChildCount(of: element) > 1 {
            return element
        }
        let firstOtherElement = element.children(matching: .other).firstMatch
        if firstOtherElement.exists {
            return try findRecoveryElement(firstOtherElement)
        } else {
            return element
        }
    }

    private func elementHierarchy(xcuiElement: XCUIElement) throws -> AXElement {
        let snapshot = try xcuiElement.snapshot()
        var hierarchy = AXElement(snapshot.dictionaryRepresentation)
        hierarchy.applyKeyboardFocus(from: snapshot)
        return hierarchy.resolvingNestedWindowOffsets()
    }
}
