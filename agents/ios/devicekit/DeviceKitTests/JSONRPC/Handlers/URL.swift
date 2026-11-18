import Foundation
import os

struct URLRequest: Codable {
    let url: String
}

@MainActor
struct URLMethodHandler: RPCMethodHandler {
    static let methodName = "device.url"

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: String(describing: Self.self)
    )

    func execute(params: JSONValue?) async throws -> JSONValue {
        let request = try decodeParams(URLRequest.self, from: params)

        guard let url = URL(string: request.url) else {
            throw RPCMethodError.invalidParams("Invalid URL format: '\(request.url)'")
        }

        do {
            let start = Date()
            try await openURL(url)
            let duration = Date().timeIntervalSince(start)
            logger.info("URL open took \(duration)s for: \(request.url)")
            return .object(["success": .bool(true)])
        } catch {
            logger.error("Error opening URL: \(error)")
            throw RPCMethodError.internalError("Error opening URL: \(error.localizedDescription)")
        }
    }

    private func openURL(_ url: URL) async throws {
        let sessionClass: AnyClass = NSClassFromString("XCTRunnerDaemonSession")!
        let sharedSelector = NSSelectorFromString("sharedSession")
        let sharedImp = sessionClass.method(for: sharedSelector)
        typealias SharedMethod = @convention(c) (AnyClass, Selector) -> NSObject
        let sharedMethod = unsafeBitCast(sharedImp, to: SharedMethod.self)
        let session = sharedMethod(sessionClass, sharedSelector)

        let openSelector = NSSelectorFromString("openDefaultApplicationForURL:completion:")
        let openImp = session.method(for: openSelector)
        typealias OpenMethod = @convention(c) (
            NSObject,
            Selector,
            NSURL,
            @escaping (Bool, Error?) -> Void
        ) -> Void
        let openMethod = unsafeBitCast(openImp, to: OpenMethod.self)

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            openMethod(session, openSelector, url as NSURL) { success, error in
                if let error = error {
                    continuation.resume(throwing: error)
                } else if !success {
                    continuation.resume(throwing: NSError(
                        domain: "URLMethodHandler",
                        code: 1,
                        userInfo: [NSLocalizedDescriptionKey: "Failed to open URL"]
                    ))
                } else {
                    continuation.resume()
                }
            }
        }
    }
}
