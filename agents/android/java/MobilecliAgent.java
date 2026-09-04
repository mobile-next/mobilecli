package com.mobilenext.mobilecli;

public class MobilecliAgent {
	public static void start() {
		AndroidBridge.init();
		AndroidBridge.sMainHandler.postDelayed(MobilecliAgent::startRpcServer, 500);
	}

	private static void startRpcServer() {
		String socket = "mobilecli." + AndroidBridge.getPackageName();
		new JsonRpcSocketServer(socket, new JsonRpcDispatcher()).startDaemon();
	}
}
