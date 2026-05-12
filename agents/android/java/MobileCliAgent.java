package com.mobilenext.mobilecli;

public class MobileCliAgent {
	public static void start() {
		AndroidBridge.init();
		AndroidBridge.sMainHandler.postDelayed(HttpRpcServer::start, 500);
	}
}
