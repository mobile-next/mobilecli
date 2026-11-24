import FlyingFox
import Foundation

@MainActor
struct JSONRPCHTTPHandler: HTTPHandler {

    private let dispatcher: JSONRPCDispatcher

    init(dispatcher: JSONRPCDispatcher) {
        self.dispatcher = dispatcher
    }

    func handleRequest(_ request: HTTPRequest) async throws -> HTTPResponse {
        let bodyData = try await request.bodyData
        NSLog("Received HTTP JSON-RPC request: \(String(data: bodyData, encoding: .utf8)?.prefix(200) ?? "")...")

        let responseData = await dispatcher.dispatch(bodyData)

        NSLog("Sending HTTP JSON-RPC response: \(String(data: responseData, encoding: .utf8)?.prefix(200) ?? "")...")

        return HTTPResponse(
            statusCode: .ok,
            headers: [.contentType: "application/json"],
            body: responseData
        )
    }
}

