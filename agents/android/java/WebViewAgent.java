package com.mobilenext.mobilecli;

import android.content.res.Resources;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.WebView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;
import org.json.JSONTokener;

import java.util.List;
import java.util.function.Consumer;
import java.util.stream.Collectors;

class WebViewAgent {

	private static final int MAX_DEPTH = 50;

	/* ── WebView lookup ──────────────────────────────────────────────────── */

	static WebView lookupWebView(String id) throws Exception {
		return AndroidBridge.getRootViews().stream()
			.flatMap(AndroidBridge::streamWebViews)
			.filter(v -> Integer.toHexString(System.identityHashCode(v)).equals(id))
			.findFirst()
			.orElseThrow(() -> new RpcException(RpcException.WEBVIEW_NOT_FOUND, "webview not found: " + id));
	}

	static WebView findWebViewById(String id) throws Exception {
		return AndroidBridge.runOnMainThread(() -> lookupWebView(id));
	}

	/* ── Navigation ──────────────────────────────────────────────────────── */

	static void webViewAction(String id, Consumer<WebView> action) throws Exception {
		AndroidBridge.runOnMainThread(() -> {
			action.accept(lookupWebView(id));
			return null;
		});
	}

	static Consumer<WebView> navAction(String method) {
		switch (method) {
			case "device.webview.reload":
				return WebView::reload;
			case "device.webview.goBack":
				return WebView::goBack;
			default:
				return WebView::goForward;
		}
	}

	/* ── device.webview.waitForLoadState ─────────────────────────────────── */

	static void waitForLoadState(WebView wv, String state, int timeoutMs) throws Exception {
		String js = "domcontentloaded".equals(state)
			? "document.readyState === 'interactive' || document.readyState === 'complete'"
			: "document.readyState === 'complete'";
		long deadline = System.currentTimeMillis() + timeoutMs;
		while (true) {
			String raw = AndroidBridge.evalJs(wv, "String(" + js + ")");
			if ("true".equals(raw) || "\"true\"".equals(raw)) {
				return;
			}
			if (System.currentTimeMillis() >= deadline) {
				throw new Exception("waitForLoadState timed out waiting for '" + state + "'");
			}
			Thread.sleep(200);
		}
	}

	/* ── device.webview.evaluate ─────────────────────────────────────────── */

	private static long sEvalCounter = 0;

	static JSONObject evaluateExpression(WebView wv, String expression, JSONArray args) throws Exception {
		String argsJson = (args != null && args.length() > 0) ? args.toString() : "[]";
		// WebView.evaluateJavascript does not await promises — it would serialize a
		// returned Promise to "{}". So kick off the (possibly async) expression,
		// stash its awaited result on a window slot keyed by a token, then poll for
		// it. This makes async expressions (e.g. Playwright's injected expect())
		// resolve correctly instead of yielding an empty object.
		String token = "mw" + (sEvalCounter++);
		AndroidBridge.evalJs(wv, buildEvalScript(expression, argsJson, token));

		String outcomeJson = null;
		long deadline = System.currentTimeMillis() + 10_000;
		while (System.currentTimeMillis() < deadline) {
			String raw = AndroidBridge.evalJs(wv, buildPollScript(token));
			if (raw != null && !"null".equals(raw)) {
				Object tokenized = new JSONTokener(raw).nextValue();
				if (tokenized instanceof String) {
					outcomeJson = (String) tokenized;
					break;
				}
			}
			Thread.sleep(20);
		}
		if (outcomeJson == null) {
			throw new Exception("script execution timed out waiting for result");
		}
		JSONObject outcome = new JSONObject(outcomeJson);
		if (!outcome.optBoolean("ok", false)) {
			throw new Exception(outcome.optString("error", "script error"));
		}
		Object value = outcome.isNull("value") ? JSONObject.NULL : outcome.get("value");
		return new JSONObject().put("result", value);
	}

	private static String buildEvalScript(String expression, String argsJson, String token) {
		// Wrap bare expressions (no return / no statement separator / no block) so
		// callers can pass "document.title" instead of "return document.title".
		String trimmed = expression.trim();
		boolean looksLikeStatement = trimmed.startsWith("return ")
			|| trimmed.contains(";")
			|| trimmed.contains("\n")
			|| trimmed.startsWith("{");
		String body = looksLikeStatement ? trimmed : "return (" + trimmed + ")";
		// Promise.resolve() handles both sync values and thenables uniformly, so a
		// returned promise is awaited before the result is serialized.
		return "(function() {" +
			"  window.__mwEval = window.__mwEval || {};" +
			"  var __args = " + argsJson + ";" +
			"  try {" +
			"    var __r = (function() { " + body + " }).apply(null, __args);" +
			"    Promise.resolve(__r).then(function(v) {" +
			"      window.__mwEval['" + token + "'] = JSON.stringify({ ok: true, value: (v === undefined ? null : v) });" +
			"    }, function(e) {" +
			"      window.__mwEval['" + token + "'] = JSON.stringify({ ok: false, error: (e && e.message) || String(e) });" +
			"    });" +
			"  } catch(e) {" +
			"    window.__mwEval['" + token + "'] = JSON.stringify({ ok: false, error: (e && e.message) || String(e) });" +
			"  }" +
			"})()";
	}

	// Read and clear the stored result for a token; returns null until it's ready.
	private static String buildPollScript(String token) {
		return "(function() {" +
			"  var m = window.__mwEval;" +
			"  if (!m) { return null; }" +
			"  var r = m['" + token + "'];" +
			"  if (r === undefined) { return null; }" +
			"  delete m['" + token + "'];" +
			"  return r;" +
			"})()";
	}

	/* ── device.dump.ui ──────────────────────────────────────────────────── */

	static JSONArray dumpUi() throws Exception {
		return AndroidBridge.runOnMainThread(() -> {
			List<View> roots = AndroidBridge.getRootViews();
			JSONArray arr = new JSONArray();
			for (View root : roots) {
				arr.put(viewToJson(root, 0));
			}
			return arr;
		});
	}

	private static JSONObject viewToJson(View view, int depth) throws Exception {
		JSONObject obj = new JSONObject();
		obj.put("type", view.getClass().getName());

		int id = view.getId();
		if (id != View.NO_ID) {
			try {
				Resources res = view.getResources();
				obj.put("label", res.getResourceEntryName(id));
				obj.put("identifier", res.getResourceName(id));
			} catch (Exception ignored) {
			}
		}

		if (view instanceof TextView) {
			CharSequence cs = ((TextView) view).getText();
			if (cs != null && cs.length() > 0) {
				obj.put("text", cs.toString());
			}
		}

		int[] loc = new int[2];
		view.getLocationOnScreen(loc);
		JSONObject rect = new JSONObject();
		rect.put("x", loc[0]);
		rect.put("y", loc[1]);
		rect.put("width", view.getWidth());
		rect.put("height", view.getHeight());
		obj.put("rect", rect);

		if (depth < MAX_DEPTH && view instanceof ViewGroup) {
			ViewGroup vg = (ViewGroup) view;
			int n = vg.getChildCount();
			if (n > 0) {
				JSONArray children = new JSONArray();
				for (int i = 0; i < n; i++) {
					children.put(viewToJson(vg.getChildAt(i), depth + 1));
				}
				obj.put("children", children);
			}
		}

		return obj;
	}

	/* ── device.webview.list ─────────────────────────────────────────────── */

	static JSONArray listWebViews() throws Exception {
		return AndroidBridge.runOnMainThread(() -> {
			List<WebView> found = AndroidBridge.getRootViews().stream()
				.flatMap(AndroidBridge::streamWebViews)
				.collect(Collectors.toList());
			JSONArray arr = new JSONArray();
			for (WebView wv : found) {
				arr.put(webViewToJson(wv));
			}
			return arr;
		});
	}

	private static JSONObject webViewToJson(WebView wv) throws Exception {
		String pkg = AndroidBridge.getPackageName();
		String id = Integer.toHexString(System.identityHashCode(wv));
		int[] loc = new int[2];
		wv.getLocationOnScreen(loc);
		int w = wv.getWidth(), h = wv.getHeight();

		JSONObject bounds = new JSONObject();
		bounds.put("x", loc[0]);
		bounds.put("y", loc[1]);
		bounds.put("width", w);
		bounds.put("height", h);

		return new JSONObject()
			.put("id", id)
			.put("url", wv.getUrl() != null ? wv.getUrl() : "")
			.put("title", wv.getTitle() != null ? wv.getTitle() : "")
			.put("bundleId", pkg)
			.put("processName", pkg)
			.put("bounds", bounds)
			.put("isVisible", w > 0 && h > 0);
	}
}
