package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.os.Looper;
import android.os.SystemClock;
import android.view.KeyEvent;
import android.view.accessibility.AccessibilityWindowInfo;

import android.util.Base64;

import org.json.JSONObject;

/**
 * Persistent device server via app_process. Keeps one connected UiAutomation
 * alive and serves UI dump, screenshot, keyboard, clipboard, app list and mock location
 * over JSON-RPC on a localabstract socket, so repeated calls skip process-fork
 * and connect cost, and long-lived state (test location providers) has a home.
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex nohup app_process / \
 *     com.mobilenext.mobilecli.UiDumpServer &
 *   adb forward tcp:0 localabstract:mobilecli-uidump
 *
 * Must be run as the shell or root user.
 */
public class UiDumpServer {

	static final String SOCKET_NAME = "mobilecli-uidump";

	public static void main(String[] args) {
		try {
			UiAutomation automation = UiAutomationFactory.createAndConnect();
			UiAutomationFactory.configureForWindowRetrieval(automation);

			new JsonRpcSocketServer(SOCKET_NAME, (method, params) -> {
				switch (method) {
					case "device.dump.ui": {
						long waitUntilIdle = params == null ? 0 : params.optLong("waitUntilIdle", 0);
						return UiTreeSerializer.dump(automation, waitUntilIdle);
					}
					case "device.screenshot": {
						JSONObject p = params == null ? new JSONObject() : params;
						int[] clip = null;
						JSONObject c = p.optJSONObject("clip");
						if (c != null) {
							clip = new int[]{c.getInt("x"), c.getInt("y"), c.getInt("width"), c.getInt("height")};
						}
						byte[] image = Screenshot.capture(automation, p.optString("format", "png"),
								p.optInt("quality", 90), p.optDouble("scale", 1.0), p.optInt("maxSize", 0),
								clip, p.optInt("screenWidth", 0));
						return new JSONObject().put("data", Base64.encodeToString(image, Base64.NO_WRAP));
					}
					case "device.io.keyboard.hide":
						return new JSONObject().put("dismissed", hideKeyboard(automation));
					case "device.clipboard.get": {
						String text = Clipboard.getText();
						return new JSONObject().put("text", text == null ? "" : text);
					}
					case "device.clipboard.set":
						Clipboard.setText(JsonRpcDispatcher.requireParam(params, "text"));
						return new JSONObject().put("ok", true);
					case "device.clipboard.clear":
						Clipboard.clear();
						return new JSONObject().put("ok", true);
					case "device.apps.list":
						return PackageLister.listPackages();
					case "device.location.set": {
						if (params == null || !params.has("lat") || !params.has("lon")) {
							throw new RpcException(RpcException.INVALID_PARAMS, "missing params.lat/lon");
						}
						MockLocation.start(params.getDouble("lat"), params.getDouble("lon"));
						return new JSONObject().put("ok", true);
					}
					case "device.location.clear":
						MockLocation.clear();
						return new JSONObject().put("ok", true);
					default:
						throw new RpcException(RpcException.METHOD_NOT_FOUND, "Method not found: " + method);
				}
			}).startDaemon();

			// the accessibility framework posts callbacks to the main looper
			Looper.loop();
		} catch (Throwable e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

	// The soft keyboard is just an IME-type window; if it's up, BACK dismisses it.
	// ponytail: no headless way to *show* the IME — Android only raises it for a
	// focused editable view in the target app, so only hide is offered here.
	private static boolean hideKeyboard(UiAutomation automation) {
		boolean imeShown = false;
		for (AccessibilityWindowInfo w : automation.getWindows()) {
			if (w.getType() == AccessibilityWindowInfo.TYPE_INPUT_METHOD) imeShown = true;
		}
		if (!imeShown) return false;

		long now = SystemClock.uptimeMillis();
		automation.injectInputEvent(new KeyEvent(now, now, KeyEvent.ACTION_DOWN, KeyEvent.KEYCODE_BACK, 0), true);
		automation.injectInputEvent(new KeyEvent(now, now, KeyEvent.ACTION_UP, KeyEvent.KEYCODE_BACK, 0), true);
		return true;
	}
}
