package com.mobilenext.mobilecli;

import android.net.LocalServerSocket;
import android.net.LocalSocket;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.io.PrintWriter;

class HttpRpcServer {

	static void start() {
		String name = "mobilecli." + AndroidBridge.getPackageName();
		new Thread(() -> {
			try {
				LocalServerSocket server = new LocalServerSocket(name);
				android.util.Log.d("MobileCliAgent", "listening on localabstract:" + name);
				while (true) {
					handleClient(server.accept());
				}
			} catch (Exception e) {
				android.util.Log.e("MobileCliAgent", "server error: " + e.getMessage());
			}
		}, "vta-server").start();
	}

	private static void handleClient(LocalSocket client) {
		new Thread(() -> {
			try (
				BufferedReader in = new BufferedReader(new InputStreamReader(client.getInputStream()));
				PrintWriter out = new PrintWriter(client.getOutputStream(), false)
			) {
				String requestLine = in.readLine();
				if (requestLine == null) {
					return;
				}

				int contentLength = 0;
				String header;
				while ((header = in.readLine()) != null && !header.isEmpty()) {
					if (header.toLowerCase().startsWith("content-length:")) {
						contentLength = Integer.parseInt(header.substring(15).trim());
					}
				}

				char[] body = new char[contentLength];
				if (contentLength > 0) {
					in.read(body, 0, contentLength);
				}
				String bodyStr = new String(body).trim();

				String response = JsonRpcDispatcher.dispatch(bodyStr.isEmpty() ? "{}" : bodyStr);

				byte[] bytes = response.getBytes("UTF-8");
				out.print("HTTP/1.1 200 OK\r\n");
				out.print("Content-Type: application/json\r\n");
				out.print("Content-Length: " + bytes.length + "\r\n");
				out.print("Connection: close\r\n");
				out.print("\r\n");
				out.print(response);
				out.flush();
			} catch (Exception e) {
				android.util.Log.e("MobileCliAgent", "client error: " + e.getMessage());
			}
		}, "vta-client").start();
	}
}
