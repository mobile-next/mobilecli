package com.mobilenext.mobilecli;

import android.webkit.WebView;

import org.json.JSONArray;
import org.json.JSONObject;

/**
 * The agent injected into a target app by mobilecli.so (JVMTI). Runs inside
 * the app's process, so it can reach its WebView instances and Flutter engine;
 * everything device-wide lives in DeviceServer instead. Serves JSON-RPC on
 * localabstract:mobilecli.<package>.
 */
public class InjectedAgent implements JsonRpcSocketServer.Dispatcher {

	// entry point called by jvmti_agent.c once the app's main looper is up
	public static void start() {
		AndroidBridge.init();
		AndroidBridge.sMainHandler.postDelayed(InjectedAgent::startRpcServer, 500);
	}

	private static void startRpcServer() {
		String socket = "mobilecli." + AndroidBridge.getPackageName();
		new JsonRpcSocketServer(socket, new InjectedAgent()).startDaemon();
	}

	@Override
	public Object dispatch(String method, JSONObject params) throws Exception {
		switch (method) {

			case "device.dump.ui":
				return WebViews.dumpUi();

			case "device.flutter.vmServiceUri":
				return flutterVmServiceUri();

			case "device.webview.list":
				return WebViews.listWebViews();

			case "device.webview.goto": {
				String wvId = JsonRpcSocketServer.requireParam(params, "id");
				String url = JsonRpcSocketServer.requireParam(params, "url");
				AndroidBridge.runOnMainThread(() -> {
					WebViews.lookupWebView(wvId).loadUrl(url);
					return null;
				});
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.reload":
			case "device.webview.goBack":
			case "device.webview.goForward": {
				WebViews.webViewAction(JsonRpcSocketServer.requireParam(params, "id"), WebViews.navAction(method));
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.waitForLoadState": {
				String wvId = JsonRpcSocketServer.requireParam(params, "id");
				String state = params.optString("state", "load");
				int timeout = params.optInt("timeout", 30_000);
				WebView wv = WebViews.findWebViewById(wvId);
				WebViews.waitForLoadState(wv, state, timeout);
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.evaluate": {
				String wvId = JsonRpcSocketServer.requireParam(params, "id");
				String expression = JsonRpcSocketServer.requireParam(params, "expression");
				JSONArray args = params.optJSONArray("args");
				return WebViews.evaluateExpression(WebViews.findWebViewById(wvId), expression, args);
			}

			default:
				throw new RpcException(RpcException.METHOD_NOT_FOUND, "method not found: " + method);
		}
	}

	// Returns the running Flutter app's Dart VM service URI (including the auth
	// token) by reflecting FlutterJNI.getVMServiceUri(). FlutterJNI lives in the
	// host app's PathClassLoader — reached via Context.getClassLoader() on
	// ActivityThread.currentApplication(); the agent dex's own loader (parented
	// to the system loader) cannot see it, and app.getClass().getClassLoader()
	// is the boot loader when the app uses the base Application class.
	private static JSONObject flutterVmServiceUri() throws Exception {
		Object app = Class.forName("android.app.ActivityThread")
			.getMethod("currentApplication").invoke(null);
		ClassLoader appLoader = app == null
			? Thread.currentThread().getContextClassLoader()
			: (ClassLoader) app.getClass().getMethod("getClassLoader").invoke(app);

		Class<?> jni;
		try {
			jni = Class.forName("io.flutter.embedding.engine.FlutterJNI", false, appLoader);
		} catch (ClassNotFoundException e) {
			throw new RpcException(RpcException.INVALID_PARAMS, "not a flutter app");
		}

		Object v;
		try {
			v = jni.getMethod("getVMServiceUri").invoke(null);
		} catch (NoSuchMethodException e) {
			v = jni.getMethod("getObservatoryUri").invoke(null); // pre-2022 engines
		}
		return new JSONObject().put("uri", v == null ? "" : v.toString());
	}
}
