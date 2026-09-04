package com.mobilenext.mobilecli;

import android.webkit.WebView;

import org.json.JSONArray;
import org.json.JSONObject;

/** JSON-RPC methods served by the in-app webview agent (injected via JVMTI). */
class JsonRpcDispatcher implements JsonRpcSocketServer.Dispatcher {

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

	@Override
	public Object dispatch(String method, JSONObject params) throws Exception {
		switch (method) {

			case "device.dump.ui":
				return WebViewAgent.dumpUi();

			case "device.flutter.vmServiceUri":
				return flutterVmServiceUri();

			case "device.webview.list":
				return WebViewAgent.listWebViews();

			case "device.webview.goto": {
				String wvId = requireParam(params, "id");
				String url = requireParam(params, "url");
				AndroidBridge.runOnMainThread(() -> {
					WebViewAgent.lookupWebView(wvId).loadUrl(url);
					return null;
				});
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.reload":
			case "device.webview.goBack":
			case "device.webview.goForward": {
				WebViewAgent.webViewAction(requireParam(params, "id"), WebViewAgent.navAction(method));
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.waitForLoadState": {
				String wvId = requireParam(params, "id");
				String state = params.optString("state", "load");
				int timeout = params.optInt("timeout", 30_000);
				WebView wv = WebViewAgent.findWebViewById(wvId);
				WebViewAgent.waitForLoadState(wv, state, timeout);
				return new JSONObject().put("status", "ok");
			}

			case "device.webview.evaluate": {
				String wvId = requireParam(params, "id");
				String expression = requireParam(params, "expression");
				JSONArray args = params.optJSONArray("args");
				return WebViewAgent.evaluateExpression(WebViewAgent.findWebViewById(wvId), expression, args);
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
