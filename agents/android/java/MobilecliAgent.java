package com.mobilenext.mobilecli;

public class MobilecliAgent {
	public static void start() {
		AndroidBridge.init();
		AndroidBridge.sMainHandler.postDelayed(HttpRpcServer::start, 500);
	}
}
