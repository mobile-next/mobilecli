package com.mobilenext.mobilecli;

import android.util.Base64;

import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicLong;
import java.util.regex.Pattern;

/** Request routing and the fail-open handoff between an app thread and RPC threads. */
final class Network {
	private static final long REQUEST_TIMEOUT_MS = 60_000;
	private static final long POLL_TIMEOUT_MS = 30_000;
	private static final long POLL_GRACE_MS = 15_000;
	private static final int MAX_EVENTS = 256;
	private static final int MAX_BODY_BYTES = 256 * 1024;
	private static final Object LOCK = new Object();
	private static final AtomicLong IDS = new AtomicLong();
	private static final Map<String, Pause> PAUSED = new HashMap<>();
	private static final ArrayDeque<JSONObject> EVENTS = new ArrayDeque<>();
	private static List<Route> routes = new ArrayList<>();
	private static boolean enabled;
	private static int pollers;
	private static long lastPollEnd;

	static final class Verdict {
		final String action;
		final JSONObject params;
		private Verdict(String action, JSONObject params) {
			this.action = action;
			this.params = params;
		}
	}

	private static final Verdict UNCHANGED = new Verdict("continue", new JSONObject());

	private static final class Pause {
		Verdict verdict;
		JSONObject event;
	}

	private static final class Route {
		final Pattern url;
		final String method;
		Route(Pattern url, String method) {
			this.url = url;
			this.method = method;
		}
		boolean matches(String requestUrl, String requestMethod) {
			return (method == null || method.equalsIgnoreCase(requestMethod))
				&& url.matcher(requestUrl).matches();
		}
	}

	private Network() { }

	static JSONObject enable(JSONObject params) throws Exception {
		JSONArray definitions = params == null ? null : params.optJSONArray("routes");
		if (definitions == null) {
			throw new RpcException(RpcException.INVALID_PARAMS, "routes must be an array");
		}
		List<Route> next = new ArrayList<>();
		for (int i = 0; i < definitions.length(); i++) {
			JSONObject definition = definitions.optJSONObject(i);
			if (definition == null) {
				throw new RpcException(RpcException.INVALID_PARAMS, "route must be an object");
			}
			String glob = JsonRpcSocketServer.requireParam(definition, "url");
			String method = definition.optString("method", null);
			next.add(new Route(NetworkGlob.compile(glob), method == null || method.isEmpty() ? null : method));
		}
		// An empty route list means intercept every request.
		if (next.isEmpty()) {
			next.add(new Route(Pattern.compile(".*", Pattern.DOTALL), null));
		}
		synchronized (LOCK) {
			releaseAllLocked();
			EVENTS.clear();
			routes = next;
			enabled = true;
			lastPollEnd = System.currentTimeMillis();
			LOCK.notifyAll();
		}
		return ok();
	}

	static JSONObject poll(JSONObject params) throws JSONException {
		JSONArray batch = new JSONArray();
		long deadline = System.currentTimeMillis() + POLL_TIMEOUT_MS;
		synchronized (LOCK) {
			pollers++;
			try {
				while (enabled && EVENTS.isEmpty()) {
					long remaining = deadline - System.currentTimeMillis();
					if (remaining <= 0) {
						break;
					}
					try { LOCK.wait(remaining); } catch (InterruptedException e) {
						Thread.currentThread().interrupt();
						break;
					}
				}
				while (!EVENTS.isEmpty()) batch.put(EVENTS.removeFirst());
			} finally {
				pollers--;
				lastPollEnd = System.currentTimeMillis();
			}
		}
		return new JSONObject().put("events", batch);
	}

	static JSONObject continueRequest(JSONObject params) throws Exception {
		return decide(params, "continue");
	}

	static JSONObject fulfill(JSONObject params) throws Exception {
		int status = params == null ? 0 : params.optInt("status", 0);
		if (status < 100 || status > 599) {
			throw new RpcException(RpcException.INVALID_PARAMS, "status must be an HTTP status code");
		}
		return decide(params, "fulfill");
	}

	static JSONObject abort(JSONObject params) throws Exception {
		return decide(params, "abort");
	}

	static JSONObject disable(JSONObject params) throws JSONException {
		synchronized (LOCK) {
			enabled = false;
			routes = new ArrayList<>();
			EVENTS.clear();
			releaseAllLocked();
			LOCK.notifyAll();
		}
		return ok();
	}

	/** Called only by the OkHttp interceptor on the app's request thread. */
	static Verdict await(String method, String url, JSONObject headers, byte[] body) {
		Pause pause = new Pause();
		String id;
		synchronized (LOCK) {
			if (!enabled || !matchesLocked(url, method) || EVENTS.size() >= MAX_EVENTS) {
				return UNCHANGED;
			}
			id = Long.toString(IDS.incrementAndGet());
			JSONObject event = new JSONObject();
			try {
				event.put("type", "paused");
				event.put("requestId", id);
				event.put("method", method);
				event.put("url", url);
				event.put("headers", headers);
				event.put("body", body == null ? JSONObject.NULL : Base64.encodeToString(
					body, 0, Math.min(body.length, MAX_BODY_BYTES), Base64.NO_WRAP));
				if (body != null && body.length > MAX_BODY_BYTES) {
					event.put("bodyTruncated", true);
				}
				event.put("stack", "okhttp");
			} catch (JSONException e) {
				return UNCHANGED;
			}
			PAUSED.put(id, pause);
			pause.event = event;
			EVENTS.addLast(event);
			LOCK.notifyAll();
		}
		long deadline = System.currentTimeMillis() + REQUEST_TIMEOUT_MS;
		synchronized (LOCK) {
			while (pause.verdict == null) {
				long now = System.currentTimeMillis();
				if (!enabled || now >= deadline || (pollers == 0 && now - lastPollEnd >= POLL_GRACE_MS)) {
					pause.verdict = UNCHANGED;
					break;
				}
				long remaining = Math.min(deadline - now, 1000);
				try { LOCK.wait(remaining); } catch (InterruptedException e) {
					Thread.currentThread().interrupt();
					pause.verdict = UNCHANGED;
				}
			}
			PAUSED.remove(id);
			EVENTS.remove(pause.event);
			return pause.verdict;
		}
	}

	private static JSONObject decide(JSONObject params, String action) throws Exception {
		String id = JsonRpcSocketServer.requireParam(params, "requestId");
		// Check payload before waking the app thread.
		if (params.has("body") && !params.isNull("body")) {
			try { Base64.decode(params.getString("body"), Base64.DEFAULT); }
			catch (IllegalArgumentException e) {
				throw new RpcException(RpcException.INVALID_PARAMS, "body must be base64");
			}
		}
		synchronized (LOCK) {
			Pause pause = PAUSED.get(id);
			if (pause == null || pause.verdict != null) {
				throw new RpcException(RpcException.INVALID_PARAMS, "request is not paused: " + id);
			}
			pause.verdict = new Verdict(action, params);
			PAUSED.remove(id);
			LOCK.notifyAll();
		}
		return ok();
	}

	private static boolean matchesLocked(String url, String method) {
		for (Route route : routes) {
			if (route.matches(url, method)) {
				return true;
			}
		}
		return false;
	}

	private static void releaseAllLocked() {
		for (Pause pause : PAUSED.values()) {
			pause.verdict = UNCHANGED;
		}
		PAUSED.clear();
		LOCK.notifyAll();
	}

	private static JSONObject ok() throws JSONException {
		return new JSONObject().put("status", "ok");
	}
}
