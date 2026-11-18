import os

/// `fields` names the parts of the answer the caller wants. Leaving it out asks for all of
/// them, as before. `bundleId` and `pid` are always reported, since they are free; `name`
/// and `viewController` are only computed when asked for, because finding the view
/// controller means snapshotting the hierarchy, which on an app with a large one costs as
/// much as a UI dump.
struct AppsForegroundRequest: Codable {
    let fields: [String]?

    func wants(_ field: String) -> Bool {
        fields?.contains(field) ?? true
    }
}

@MainActor
struct AppsForegroundMethodHandler: RPCMethodHandler {
    static let methodName = "device.apps.foreground"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(AppsForegroundRequest.self, from: params)
        let start = Date()

        logger.info("[Start] Getting foreground app")

        let foregroundApp = RunningApp.getForegroundApp()
        guard let foregroundApp = foregroundApp else {
            let duration = Date().timeIntervalSince(start)
            logger.info("[Done] No foreground app found, took \(duration)")
            return .object(["bundleId": .string(""), "name": .string(""), "pid": .int(0)])
        }

        guard let bundleId = foregroundApp.bundleID else {
            throw RPCMethodError.internalError("No bundleID in apps_foreground found")
        }

        let pid = foregroundApp.processID
        // XCUIApplication.label resolves the element, which on an app with a large
        // hierarchy takes seconds; the accessibility server answers for the
        // application element alone in milliseconds.
        let name = request.wants("name")
            ? AXClientProxy.sharedClient().label(forProcessIdentifier: pid) ?? foregroundApp.label
            : nil
        let viewController = request.wants("viewController")
            ? AXClientProxy.sharedClient().viewControllerClassName(forProcessIdentifier: pid)
            : nil

        let duration = Date().timeIntervalSince(start)
        logger.info("[Done] Foreground app: \(bundleId) (pid: \(pid)), took \(duration)")

        var result: [String: JSONValue] = [
            "bundleId": .string(bundleId),
            "pid": .int(Int(pid))
        ]

        if let name {
            result["name"] = .string(name)
        }

        if let viewController {
            result["viewController"] = .string(viewController)
        }

        return .object(result)
    }
}
