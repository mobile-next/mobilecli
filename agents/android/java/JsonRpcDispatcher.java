package com.mobilenext.mobilecli;

import android.webkit.WebView;

import org.json.JSONArray;
import org.json.JSONObject;

class JsonRpcDispatcher {

	private static final String INTERNAL_ERROR_RESPONSE = buildInternalErrorResponse();

	private static String buildInternalErrorResponse() {
		try {
			return new JSONObject()
				.put("jsonrpc", "2.0")
				.put("error", new JSONObject()
					.put("code", RpcException.INTERNAL_ERROR)
					.put("message", "internal error"))
				.toString();
		} catch (Exception e) {
			return "";
		}
	}

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

	static String dispatch(String json) {
		String id = null;
		try {
			JSONObject req = new JSONObject(json);
			id = req.optString("id", null);
			String method = req.optString("method", "");
			JSONObject params = req.optJSONObject("params");

			switch (method) {

				case "device.dump.ui":
					return result(id, WebViewAgent.dumpUi());

				case "device.webview.list":
					return result(id, WebViewAgent.listWebViews());

				case "device.webview.goto": {
					String wvId = requireParam(params, "id");
					String url = requireParam(params, "url");
					AndroidBridge.runOnMainThread(() -> {
						WebViewAgent.lookupWebView(wvId).loadUrl(url);
						return null;
					});
					return result(id, new JSONObject().put("status", "ok"));
				}

				case "device.webview.reload":
				case "device.webview.goBack":
				case "device.webview.goForward": {
					WebViewAgent.webViewAction(requireParam(params, "id"), WebViewAgent.navAction(method));
					return result(id, new JSONObject().put("status", "ok"));
				}

				case "device.webview.waitForLoadState": {
					String wvId = requireParam(params, "id");
					String state = params != null ? params.optString("state", "load") : "load";
					int timeout = params != null ? params.optInt("timeout", 30_000) : 30_000;
					WebView wv = WebViewAgent.findWebViewById(wvId);
					WebViewAgent.waitForLoadState(wv, state, timeout);
					return result(id, new JSONObject().put("status", "ok"));
				}

				case "device.webview.evaluate": {
					String wvId = requireParam(params, "id");
					String expression = requireParam(params, "expression");
					JSONArray args = params.optJSONArray("args");
					return result(id, WebViewAgent.evaluateExpression(WebViewAgent.findWebViewById(wvId), expression, args));
				}

				default:
					return error(id, RpcException.METHOD_NOT_FOUND, "method not found: " + method);
			}
		} catch (RpcException e) {
			return error(id, e.code, e.getMessage());
		} catch (Exception e) {
			return error(id, RpcException.SERVER_ERROR, e.getMessage());
		}
	}

	private static String result(String id, Object value) {
		try {
			return new JSONObject()
				.put("jsonrpc", "2.0")
				.put("id", id)
				.put("result", value)
				.toString();
		} catch (Exception e) {
			return INTERNAL_ERROR_RESPONSE;
		}
	}

	private static String error(String id, int code, String message) {
		try {
			JSONObject err = new JSONObject()
				.put("code", code)
				.put("message", message);
			JSONObject r = new JSONObject()
				.put("jsonrpc", "2.0")
				.put("error", err);
			if (id != null) {
				r.put("id", id);
			}
			return r.toString();
		} catch (Exception e) {
			return INTERNAL_ERROR_RESPONSE;
		}
	}
}
