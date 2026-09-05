package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.os.Looper;
import android.os.SystemClock;
import android.view.KeyEvent;
import android.view.accessibility.AccessibilityWindowInfo;

import android.util.Base64;

import org.json.JSONObject;

import java.io.FileInputStream;
import java.io.InputStream;
import java.security.MessageDigest;

/**
 * Persistent device server via app_process. Keeps one connected UiAutomation
 * alive and serves UI dump, screenshot, keyboard, clipboard, app list and mock location
 * over JSON-RPC on a localabstract socket, so repeated calls skip process-fork
 * and connect cost, and long-lived state (test location providers) has a home.
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex nohup app_process / \
 *     com.mobilenext.mobilecli.DeviceServer &
 *   adb forward tcp:0 localabstract:mobilecli-server
 *
 * Must be run as the shell or root user.
 */
public class DeviceServer {

	static final String SOCKET_NAME = "mobilecli-server";

	// sha256 of the dex this process was started from, so the host can tell a
	// server left over from an older mobilecli build and restart it
	private static final String DEX_SHA256 = hashClassPath();

	public static void main(String[] args) {
		try {
			UiAutomation automation = UiAutomationFactory.createAndConnect();
			UiAutomationFactory.configureForWindowRetrieval(automation);

			new JsonRpcSocketServer(SOCKET_NAME, (method, params) -> dispatch(automation, method, params)).startDaemon();

			// the accessibility framework posts callbacks to the main looper
			Looper.loop();
		} catch (Throwable e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

	private static Object dispatch(UiAutomation automation, String method, JSONObject params) throws Exception {
		JSONObject p = params == null ? new JSONObject() : params;
		switch (method) {
			case "device.version":
				return new JSONObject().put("dexSha256", DEX_SHA256);
			case "device.dump.ui":
				return UiTreeSerializer.dump(automation, p.optLong("waitUntilIdle", 0));
			case "device.screenshot":
				return screenshot(automation, p);
			case "device.io.keyboard.hide":
				return new JSONObject().put("dismissed", hideKeyboard(automation));
			case "device.clipboard.get":
				return new JSONObject().put("text", orEmpty(Clipboard.getText()));
			case "device.clipboard.set":
				Clipboard.setText(JsonRpcSocketServer.requireParam(params, "text"));
				return ok();
			case "device.clipboard.clear":
				Clipboard.clear();
				return ok();
			case "device.apps.list":
				return PackageLister.listPackages();
			case "device.location.set":
				MockLocation.start(requireDouble(p, "lat"), requireDouble(p, "lon"));
				return ok();
			case "device.location.clear":
				MockLocation.clear();
				return ok();
			default:
				throw new RpcException(RpcException.METHOD_NOT_FOUND, "Method not found: " + method);
		}
	}

	private static JSONObject screenshot(UiAutomation automation, JSONObject p) throws Exception {
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

	private static JSONObject ok() throws Exception {
		return new JSONObject().put("ok", true);
	}

	private static String orEmpty(String s) {
		return s == null ? "" : s;
	}

	private static double requireDouble(JSONObject p, String key) throws RpcException {
		if (!p.has(key)) {
			throw new RpcException(RpcException.INVALID_PARAMS, "missing params." + key);
		}
		return p.optDouble(key);
	}

	private static String hashClassPath() {
		try (InputStream in = new FileInputStream(System.getProperty("java.class.path"))) {
			MessageDigest digest = MessageDigest.getInstance("SHA-256");
			byte[] buf = new byte[8192];
			int n;
			while ((n = in.read(buf)) > 0) digest.update(buf, 0, n);
			StringBuilder hex = new StringBuilder();
			for (byte b : digest.digest()) hex.append(String.format("%02x", b));
			return hex.toString();
		} catch (Exception e) {
			return "";
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
