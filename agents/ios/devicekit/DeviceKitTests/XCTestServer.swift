import FlyingFox
import Foundation
import os

extension String {
    /// Converts the string to a UInt16 port number.
    /// - Returns: The port number if conversion succeeds, `nil` otherwise.
    func toUInt16() -> UInt16? {
        return UInt16(self)
    }
}

/// Set by `/shutdown`. FlyingFox's `run()` throws once `stop()` closes the socket,
/// so this is how `start()` tells a requested shutdown apart from a real failure.
private final class ShutdownRequest: @unchecked Sendable {
    private let lock = NSLock()
    private var requested = false

    var isRequested: Bool {
        lock.lock()
        defer { lock.unlock() }
        return requested
    }

    func request() {
        lock.lock()
        defer { lock.unlock() }
        requested = true
    }
}

// MARK: - WebSocket HTTP & JSON-RPC Server

/// WebSocket server with JSON-RPC 2.0 protocol for UI automation.
///
/// This server provides a WebSocket endpoint that accepts JSON-RPC requests
/// and returns JSON-RPC responses for programmatic device control.
///
/// ## Configuration
/// - **Host**: `127.0.0.1` (localhost only)
/// - **Port**: `12004` (configurable via `PORT` environment variable)
/// - **Endpoint**: `ws://127.0.0.1:12004/ws`
///
/// ## Starting the Server
/// ```swift
/// let server = XCTestServer()
/// try await server.start()  // Blocks until shutdown
/// ```
///
/// ## JSON-RPC Methods
///
/// ### io_tap
/// Performs a tap or long-press at specified coordinates.
/// ```json
/// // Request
/// {"jsonrpc": "2.0", "method": "io_tap", "params": {"x": 100.0, "y": 200.0}, "id": 1}
///
/// // Response
/// {"jsonrpc": "2.0", "result": {"success": true}, "id": 1}
/// ```
///
/// ### dump_ui
/// Returns the complete view hierarchy.
/// ```json
/// // Request
/// {"jsonrpc": "2.0", "method": "dump_ui", "params": {"appIds": [], "excludeKeyboardElements": false}, "id": 2}
///
/// // Response
/// {"jsonrpc": "2.0", "result": {"axElement": {...}, "depth": 15}, "id": 2}
/// ```
///
/// ## Client Example (JavaScript)
/// ```javascript
/// const ws = new WebSocket('ws://127.0.0.1:12004/ws');
/// ws.onopen = () => {
///     ws.send(JSON.stringify({
///         jsonrpc: '2.0',
///         method: 'io_tap',
///         params: { x: 100, y: 200 },
///         id: 1
///     }));
/// };
/// ws.onmessage = (event) => {
///     const response = JSON.parse(event.data);
///     console.log(response);
/// };
/// ```
@MainActor
final class XCTestServer {

    /// Default timeout for WebSocket operations.
    private let defaultTimeout: TimeInterval = 100

    /// Default port for the WebSocket server.
    private let defaultPort: UInt16 = 12004

    /// Server binds to localhost only by default. Override with DEVICEKIT_LISTEN_HOST: a comma-separated
    /// list of IPv4/IPv6 literals, one listener each (e.g. `127.0.0.1,fdc0::1` keeps loopback alongside IPv6).
    private let listenHosts: [String] = {
        let hosts = (ProcessInfo.processInfo.environment["DEVICEKIT_LISTEN_HOST"] ?? "")
            .split(separator: ",")
            .map { $0.trimmingCharacters(in: .whitespaces) }
            .filter { !$0.isEmpty }
        // Drop exact duplicates: a second bind on the same address would fail startup.
        var seen = Set<String>()
        let unique = hosts.filter { seen.insert($0).inserted }
        return unique.isEmpty ? ["127.0.0.1"] : unique
    }()

    private let logger = Logger(
        subsystem: Bundle.main.bundleIdentifier!,
        category: "XCTestServer"
    )

    /// JSON-RPC dispatcher for routing method calls.
    private let dispatcher: JSONRPCDispatcher

    /// Initializes the WebSocket server.
    init() {
        self.dispatcher = JSONRPCDispatcher()
    }

    /// Starts the WebSocket server and blocks until shutdown.
    ///
    /// The server creates a WebSocket endpoint at `/rpc` that processes
    /// JSON-RPC requests and returns responses.
    ///
    /// - Throws: An error if the server fails to bind or encounters a runtime error.
    func start() async throws {
        let port = ProcessInfo.processInfo.environment["DEVICEKIT_LISTEN_PORT"]?.toUInt16() ?? defaultPort
        let servers = try listenHosts.map { try makeServer(host: $0, port: port) }
        let shutdown = ShutdownRequest()

        for server in servers {
            await configureRoutes(on: server, stopping: servers, shutdown: shutdown)
        }

        // Returns once every listener has stopped; if one fails, the rest are cancelled and the error propagates.
        try await withThrowingTaskGroup(of: Void.self) { group in
            for (host, server) in zip(listenHosts, servers) {
                group.addTask { try await server.run() }
                group.addTask { await self.logWhenListening(server, host: host, port: port) }
            }
            do {
                while try await group.next() != nil {}
            } catch {
                group.cancelAll()
                if shutdown.isRequested {
                    return
                }
                throw error
            }
        }
    }

    /// Logs "Server is ready" only once the socket is actually bound, so a failed bind never claims readiness.
    private func logWhenListening(_ server: HTTPServer, host: String, port: UInt16) async {
        try? await server.waitUntilListening()
        guard await server.isListening else {
            return
        }
        let displayHost = host.contains(":") ? "[\(host)]" : host
        let base = "\(displayHost):\(port)"
        logger.info("Server is ready (WebSocket: ws://\(base)/ws, HTTP: POST http://\(base)/rpc, MJPEG: http://\(base)/mjpeg)")
    }

    /// Builds an HTTPServer for one IPv4/IPv6 literal. `.inet(ip4:)` rejects IPv6 literals such as the Xcode CoreDevice tunnel address, so pick by address family.
    private func makeServer(host: String, port: UInt16) throws -> HTTPServer {
        if host.contains(":") {
            return HTTPServer(address: try .inet6(ip6: host, port: port), timeout: defaultTimeout)
        }
        return HTTPServer(address: try .inet(ip4: host, port: port), timeout: defaultTimeout)
    }

    /// Registers all routes on `server`; `/shutdown` stops every server in `servers`.
    private func configureRoutes(on server: HTTPServer, stopping servers: [HTTPServer], shutdown: ShutdownRequest) async {
        // WebSocket endpoint for JSON-RPC
        let messageHandler = JSONRPCMessageHandler(dispatcher: dispatcher)
        let frameHandler = MessageFrameWSHandler(handler: messageHandler)
        let wsHandler = WebSocketHTTPHandler(handler: frameHandler)
        await server.appendRoute("GET /ws", to: wsHandler)

        // HTTP POST endpoint for JSON-RPC
        let httpHandler = JSONRPCHTTPHandler(dispatcher: dispatcher)
        await server.appendRoute("POST /rpc", to: httpHandler)

        // Health check endpoint (HTTP)
        await server.appendRoute("GET /health") { _ in
            HTTPResponse(statusCode: .ok, body: Data("OK".utf8))
        }

        // Shutdown endpoint — stops every listener gracefully
        await server.appendRoute("POST /shutdown") { _ in
            shutdown.request()
            Task { for server in servers { await server.stop() } }
            return HTTPResponse(statusCode: .ok, body: Data("OK".utf8))
        }

        // MJPEG streaming endpoint
        let mjpegHandler = MJPEGHTTPHandler()
        await server.appendRoute("GET /mjpeg", to: mjpegHandler)
    }
}

