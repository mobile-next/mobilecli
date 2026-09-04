package com.mobilenext.mobilecli;

import android.net.LocalServerSocket;
import android.net.LocalSocket;
import android.util.Log;

import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;

/**
 * Serves JSON-RPC 2.0 over a localabstract socket, one request per connection
 * framed as HTTP/1.1 (Content-Length + body). The host reaches it with
 * `adb forward tcp:N localabstract:<name>` and a plain POST.
 *
 * Shared by the standalone app_process tools that need a control channel
 * (UiDumpServer, AvcServer); only the dispatched methods differ.
 */
class JsonRpcSocketServer {

	interface Dispatcher {
		Object dispatch(String method, JSONObject params) throws Exception;
	}

	private static final String TAG = "JsonRpcSocketServer";

	/** A non-empty string param, or INVALID_PARAMS. */
	static String requireParam(JSONObject params, String key) throws RpcException {
		if (params == null) {
			throw new RpcException(RpcException.INVALID_PARAMS, "missing params");
		}
		String v = params.optString(key, null);
		if (v == null || v.isEmpty()) {
			throw new RpcException(RpcException.INVALID_PARAMS, "missing params." + key);
		}
		return v;
	}
	private static final String JSONRPC_VERSION = "2.0";
	private static final String CONTENT_LENGTH_PREFIX = "content-length:";

	// A larger body is rejected before allocation, and a stalled peer can't pin
	// the accept thread past the read timeout.
	private static final int MAX_CONTENT_LENGTH = 1 << 20; // 1 MiB
	private static final int READ_TIMEOUT_MS = 5_000;

	private final String socketName;
	private final Dispatcher dispatcher;
	private volatile boolean running = true;
	private LocalServerSocket serverSocket;

	JsonRpcSocketServer(String socketName, Dispatcher dispatcher) {
		this.socketName = socketName;
		this.dispatcher = dispatcher;
	}

	/** Binds and serves on the calling thread until stop() or a socket error. */
	void start() throws Exception {
		serverSocket = new LocalServerSocket(socketName);
		Log.i(TAG, "Listening on localabstract:" + socketName);
		while (running) {
			LocalSocket conn;
			try {
				conn = serverSocket.accept();
			} catch (Exception e) {
				if (running) Log.e(TAG, "accept failed on " + socketName, e);
				break;
			}
			try {
				handleConnection(conn);
			} catch (Exception e) {
				Log.e(TAG, "Error handling connection", e);
			} finally {
				try {
					conn.close();
				} catch (Exception ignored) {
				}
			}
		}
	}

	/** Runs start() on a daemon thread; a bind failure surfaces as a log line there. */
	void startDaemon() {
		Thread thread = new Thread(() -> {
			try {
				start();
			} catch (Exception e) {
				Log.w(TAG, "server on " + socketName + " stopped: " + e.getMessage());
			}
		}, "jsonrpc-" + socketName);
		thread.setDaemon(true);
		thread.start();
	}

	void stop() {
		running = false;
		try {
			if (serverSocket != null) serverSocket.close();
		} catch (Exception ignored) {
		}
	}

	private void handleConnection(LocalSocket conn) throws Exception {
		conn.setSoTimeout(READ_TIMEOUT_MS);
		BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream(), StandardCharsets.ISO_8859_1));

		if (reader.readLine() == null) return; // request line

		int contentLength = 0;
		String line;
		while ((line = reader.readLine()) != null && !line.isEmpty()) {
			if (line.toLowerCase().startsWith(CONTENT_LENGTH_PREFIX)) {
				try {
					contentLength = Integer.parseInt(line.substring(CONTENT_LENGTH_PREFIX.length()).trim());
				} catch (NumberFormatException e) {
					contentLength = 0;
				}
			}
		}

		if (contentLength < 0 || contentLength > MAX_CONTENT_LENGTH) {
			writeResponse(conn, error(null, RpcException.INVALID_REQUEST, "Content-Length out of bounds"));
			return;
		}

		char[] body = new char[contentLength];
		int offset = 0;
		while (offset < contentLength) {
			int n = reader.read(body, offset, contentLength - offset);
			if (n == -1) break;
			offset += n;
		}
		if (offset < contentLength) {
			writeResponse(conn, error(null, RpcException.INVALID_REQUEST, "Incomplete request body"));
			return;
		}

		// the reader is ISO-8859-1 so Content-Length (bytes) maps 1:1 to chars;
		// recover the original UTF-8 bytes before parsing
		byte[] raw = new String(body, 0, offset).getBytes(StandardCharsets.ISO_8859_1);
		writeResponse(conn, handleJsonRpc(new String(raw, StandardCharsets.UTF_8)));
	}

	// A null body is a notification: acknowledged at the HTTP layer with no content.
	private void writeResponse(LocalSocket conn, String json) throws Exception {
		OutputStream out = conn.getOutputStream();
		if (json == null) {
			out.write("HTTP/1.1 204 No Content\r\nConnection: close\r\n\r\n".getBytes(StandardCharsets.UTF_8));
			out.flush();
			return;
		}
		byte[] bytes = json.getBytes(StandardCharsets.UTF_8);
		String head = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n"
				+ "Content-Length: " + bytes.length + "\r\nConnection: close\r\n\r\n";
		out.write(head.getBytes(StandardCharsets.UTF_8));
		out.write(bytes);
		out.flush();
	}

	// Returns the response body, or null for a notification (no "id").
	private String handleJsonRpc(String body) {
		JSONObject request;
		try {
			request = new JSONObject(body);
		} catch (Exception e) {
			return error(null, RpcException.PARSE_ERROR, "Parse error: " + e.getMessage());
		}

		boolean isNotification = !request.has("id");
		Object id = request.opt("id");

		Object method = request.opt("method");
		if (!JSONRPC_VERSION.equals(request.optString("jsonrpc")) || !(method instanceof String) || ((String) method).isEmpty()) {
			return isNotification ? null : error(id, RpcException.INVALID_REQUEST, "Invalid Request");
		}

		Object rawParams = request.opt("params");
		if (rawParams != null && rawParams != JSONObject.NULL && !(rawParams instanceof JSONObject)) {
			return isNotification ? null : error(id, RpcException.INVALID_PARAMS, "Invalid params");
		}
		JSONObject params = rawParams instanceof JSONObject ? (JSONObject) rawParams : null;

		try {
			Object result = dispatcher.dispatch((String) method, params);
			return isNotification ? null : result(id, result);
		} catch (RpcException e) {
			return isNotification ? null : error(id, e.code, e.getMessage());
		} catch (Exception e) {
			Log.e(TAG, "Internal error", e);
			return isNotification ? null : error(id, RpcException.INTERNAL_ERROR, "Internal error: " + e.getMessage());
		}
	}

	private static String result(Object id, Object result) {
		try {
			return new JSONObject()
					.put("jsonrpc", JSONRPC_VERSION)
					.put("id", id == null ? JSONObject.NULL : id)
					.put("result", result)
					.toString();
		} catch (Exception e) {
			return error(id, RpcException.INTERNAL_ERROR, "could not serialize result");
		}
	}

	private static String error(Object id, int code, String message) {
		try {
			return new JSONObject()
					.put("jsonrpc", JSONRPC_VERSION)
					.put("id", id == null ? JSONObject.NULL : id)
					.put("error", new JSONObject().put("code", code).put("message", message == null ? "" : message))
					.toString();
		} catch (Exception e) {
			return "{\"jsonrpc\":\"2.0\",\"id\":null,\"error\":{\"code\":-32603,\"message\":\"internal error\"}}";
		}
	}
}
