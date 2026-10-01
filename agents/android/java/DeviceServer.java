package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.os.Looper;
import android.os.SystemClock;
import android.view.KeyEvent;
import android.view.accessibility.AccessibilityWindowInfo;

import android.util.Base64;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.FileInputStream;
import java.io.InputStream;
import java.security.MessageDigest;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Persistent device server via app_process. Keeps one connected UiAutomation
 * alive and serves UI dump, screenshot, keys, text, touch gestures, clipboard, app list and mock location
 * over JSON-RPC on a localabstract socket, so repeated calls skip process-fork
 * and connect cost, and long-lived state (test location providers) has a home.
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex nohup app_process / \
 *     com.mobilenext.mobilecli.DeviceServer [--idle-timeout-ms=N] &
 *   adb forward tcp:0 localabstract:mobilecli-server
 *
 * While it runs it holds the device's only UiAutomation, which kills every
 * other UiAutomator client (uiautomator dump, Appium). It exits on its own
 * after --idle-timeout-ms without a request, so a host that went away without
 * stopping it does not keep the device from other tools for good.
 *
 * Must be run as the shell or root user.
 */
public class DeviceServer {

	static final String SOCKET_NAME = "mobilecli-server";

	private static final String IDLE_TIMEOUT_ARG = "--idle-timeout-ms=";
	private static final long DEFAULT_IDLE_TIMEOUT_MS = 30 * 60 * 1000;
	private static final long IDLE_CHECK_INTERVAL_MS = 1000;

	// elapsedRealtime keeps counting while the device sleeps, so an abandoned
	// device that dozed through the timeout exits as soon as it wakes
	private static volatile long lastRequestAt = SystemClock.elapsedRealtime();

	// a request still being served is not idle time: exiting in the middle of a
	// gesture leaves its finger down in the input dispatcher for good
	private static final AtomicInteger activeRequests = new AtomicInteger();

	// sha256 of the dex this process was started from, so the host can tell a
	// server left over from an older mobilecli build and restart it
	private static final String DEX_SHA256 = hashClassPath();

	public static void main(String[] args) {
		try {
			UiAutomation automation = UiAutomationFactory.createAndConnect();
			UiAutomationFactory.configureForWindowRetrieval(automation);

			new JsonRpcSocketServer(SOCKET_NAME, (method, params) -> dispatch(automation, method, params)).startDaemon();
			startIdleWatchdog(idleTimeoutMs(args));

			// the accessibility framework posts callbacks to the main looper
			Looper.loop();
		} catch (Throwable e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

	static long idleTimeoutMs(String[] args) {
		for (String arg : args) {
			if (arg.startsWith(IDLE_TIMEOUT_ARG)) {
				try {
					long value = Long.parseLong(arg.substring(IDLE_TIMEOUT_ARG.length()));
					if (value > 0) return value;
				} catch (NumberFormatException ignored) {
					// fall through to the default
				}
			}
		}
		return DEFAULT_IDLE_TIMEOUT_MS;
	}

	// exits the process once no request has arrived for idleTimeoutMs. a daemon
	// thread, so it never keeps the process alive on its own
	private static void startIdleWatchdog(long idleTimeoutMs) {
		Thread watchdog = new Thread(() -> {
			while (true) {
				try {
					Thread.sleep(Math.min(IDLE_CHECK_INTERVAL_MS, idleTimeoutMs));
				} catch (InterruptedException e) {
					return;
				}
				if (activeRequests.get() == 0 && SystemClock.elapsedRealtime() - lastRequestAt >= idleTimeoutMs) {
					System.err.println("idle for " + idleTimeoutMs + "ms, exiting");
					System.exit(0);
				}
			}
		}, "idle-watchdog");
		watchdog.setDaemon(true);
		watchdog.start();
	}

	private static Object dispatch(UiAutomation automation, String method, JSONObject params) throws Exception {
		activeRequests.incrementAndGet();
		try {
			return dispatchMethod(automation, method, params);
		} finally {
			// the timestamp moves before the count drops, so the watchdog never
			// sees an idle server with a stale timestamp
			lastRequestAt = SystemClock.elapsedRealtime();
			activeRequests.decrementAndGet();
		}
	}

	private static Object dispatchMethod(UiAutomation automation, String method, JSONObject params) throws Exception {
		JSONObject p = params == null ? new JSONObject() : params;
		switch (method) {
			case "device.version":
				return new JSONObject().put("dexSha256", DEX_SHA256);
			case "device.dump.ui":
				return UiTreeSerializer.dump(automation, p.optLong("waitUntilIdle", 0), p.optBoolean("full", false));
			case "device.screenshot":
				return screenshot(automation, p);
			case "device.io.keyboard.hide":
				return new JSONObject().put("dismissed", hideKeyboard(automation));
			case "device.io.gesture":
				return Gestures.perform(automation, p.optJSONArray("actions"));
			case "device.io.tap":
				return Input.tap(automation, requireInt(p, "x"), requireInt(p, "y"));
			case "device.io.longpress":
				return Input.longPress(automation, requireInt(p, "x"), requireInt(p, "y"), optionalInt(p, "duration", 500));
			case "device.io.swipe":
				return Input.swipe(automation, requireInt(p, "x1"), requireInt(p, "y1"), requireInt(p, "x2"), requireInt(p, "y2"),
						optionalInt(p, "duration", 1000));
			case "device.io.button":
				return Input.pressKey(automation, JsonRpcSocketServer.requireParam(params, "button"), null);
			case "device.io.keys":
				return pressKeys(automation, p.optJSONArray("keys"));
			case "device.io.text":
				return Input.typeText(automation, JsonRpcSocketServer.requireParam(params, "text"));
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

	// keys: [{keycode: "KEYCODE_A", modifiers: ["KEYCODE_CTRL_LEFT"]}, ...], pressed in order
	private static JSONObject pressKeys(UiAutomation automation, JSONArray keys) throws Exception {
		if (keys == null || keys.length() == 0) {
			throw new RpcException(RpcException.INVALID_PARAMS, "missing params.keys");
		}
		for (int i = 0; i < keys.length(); i++) {
			JSONObject key = keys.getJSONObject(i);
			Input.pressKey(automation, JsonRpcSocketServer.requireParam(key, "keycode"), key.optJSONArray("modifiers"));
		}
		return ok();
	}

	// optInt turns null, a string or a missing key into 0, which for a coordinate
	// means silently tapping the top-left corner, and it truncates or wraps
	// anything that isn't a plain int. A value that can't be used as given is a
	// bad request instead.
	private static int requireInt(JSONObject p, String key) throws RpcException {
		Object value = p.opt(key);
		if (!(value instanceof Number)) {
			throw new RpcException(RpcException.INVALID_PARAMS, "params." + key + " must be a number");
		}
		double number = ((Number) value).doubleValue();
		if (number != Math.rint(number) || number < Integer.MIN_VALUE || number > Integer.MAX_VALUE) {
			throw new RpcException(RpcException.INVALID_PARAMS,
					"params." + key + " must be a whole number that fits in an int, got " + value);
		}
		return (int) number;
	}

	private static int optionalInt(JSONObject p, String key, int fallback) throws RpcException {
		if (!p.has(key) || p.isNull(key)) {
			return fallback;
		}
		return requireInt(p, key);
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
