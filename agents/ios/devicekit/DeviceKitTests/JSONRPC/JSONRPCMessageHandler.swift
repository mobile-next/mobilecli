import FlyingFox
import Foundation

struct JSONRPCMessageHandler: WSMessageHandler {

    private let dispatcher: JSONRPCDispatcher

    init(dispatcher: JSONRPCDispatcher) {
        self.dispatcher = dispatcher
    }

    func makeMessages(for client: AsyncStream<WSMessage>) async throws -> AsyncStream<WSMessage> {
        let dispatcher = self.dispatcher
        return AsyncStream<WSMessage> { continuation in
            let task = Task { @MainActor in
                for await message in client {
                    switch message {
                    case .text(let text):
                        NSLog("Received text message: \(text.prefix(200))...")
                        let response = await dispatcher.dispatch(text)
                        NSLog("Sending response: \(response.prefix(200))...")
                        continuation.yield(WSMessage.text(response))

                    case .data(let data):
                        NSLog("Received binary message (\(data.count) bytes)")
                        let responseData = await dispatcher.dispatch(data)
                        continuation.yield(WSMessage.data(responseData))

                    case .close(let code):
                        NSLog("WebSocket close requested with code: \(code.code)")
                        continuation.yield(WSMessage.close(code))
                        continuation.finish()
                        return
                    }
                }
                NSLog("WebSocket connection closed")
                continuation.finish()
            }
            continuation.onTermination = { @Sendable _ in
                task.cancel()
            }
        }
    }
}

