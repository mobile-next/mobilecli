package com.mobilenext.mobilecli;

import android.util.Base64;
import android.util.Log;

import org.json.JSONObject;

import java.io.IOException;
import java.lang.reflect.InvocationHandler;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Proxy;
import java.util.Iterator;

/** Reflective OkHttp interceptor so the injected dex has no OkHttp dependency. */
final class OkHttpHook {
	private static final String TAG = "OkHttpHook";
	private static volatile Object installedInterceptor;

	private OkHttpHook() { }

	static synchronized void install() {
		if (installedInterceptor != null) {
			return;
		}
		try {
			Object app = Class.forName("android.app.ActivityThread")
				.getMethod("currentApplication").invoke(null);
			if (app == null) {
				throw new IllegalStateException("application unavailable");
			}
			ClassLoader loader = (ClassLoader) app.getClass().getMethod("getClassLoader").invoke(app);
			Class<?> clientClass = Class.forName("okhttp3.OkHttpClient", false, loader);
			Class<?> interceptorClass = Class.forName("okhttp3.Interceptor", false, loader);
			Object interceptor = Proxy.newProxyInstance(loader, new Class<?>[]{interceptorClass},
				new Handler());
			if (!nativeInstall(clientClass, interceptor)) {
				throw new IllegalStateException("JVMTI getter breakpoint unavailable");
			}
			installedInterceptor = interceptor;
			Log.i(TAG, "OkHttp interceptor installed");
		} catch (ClassNotFoundException e) {
			Log.i(TAG, "OkHttp is not present in the app");
		} catch (Exception | LinkageError e) {
			Log.e(TAG, "Failed to install OkHttp interceptor", e);
		}
	}

	// The attached JVMTI library registers this native method before start().
	private static native boolean nativeInstall(Class<?> clientClass, Object interceptor);

	private static final class Handler implements InvocationHandler {
		@Override public Object invoke(Object proxy, Method method, Object[] args) throws Throwable {
			if (!"intercept".equals(method.getName())) {
				if ("hashCode".equals(method.getName())) {
					return System.identityHashCode(proxy);
				}
				if ("equals".equals(method.getName())) {
					return proxy == args[0];
				}
				return "mobilecli network interceptor";
			}
			return intercept(args[0]);
		}
	}

	static Object intercept(Object chain) throws Throwable {
		Object request = call(chain, "request");
		String url = call(request, "url").toString();
		String method = (String) call(request, "method");
		JSONObject headers = headersToJson(call(request, "headers"));
		Network.Verdict verdict = Network.await(method, url, headers, requestBodyBytes(request));
		switch (verdict.action) {
			case "fulfill":
				return syntheticResponse(request, verdict.params);
			case "abort":
				throw new IOException("mobilecli network abort: "
					+ verdict.params.optString("errorCode", "failed"));
			case "continue":
				if (verdict.params.has("url") || verdict.params.has("method")
					|| verdict.params.has("headers") || verdict.params.has("body")) {
					request = overrideRequest(request, verdict.params);
				}
				return call(chain, "proceed", request);
			default:
				throw new IOException("unknown network decision: " + verdict.action);
		}
	}

	private static JSONObject headersToJson(Object headers) throws Throwable {
		JSONObject result = new JSONObject();
		int size = ((Number) call(headers, "size")).intValue();
		for (int i = 0; i < size; i++) {
			String name = (String) call(headers, "name", i);
			String value = (String) call(headers, "value", i);
			result.put(name, value);
		}
		return result;
	}

	private static byte[] requestBodyBytes(Object request) {
		try {
			Object body = call(request, "body");
			if (body == null) {
				return null;
			}
			long length = ((Number) call(body, "contentLength")).longValue();
			if (length < 0 || length > 256 * 1024) {
				return null;
			}
			try {
				if (Boolean.TRUE.equals(call(body, "isOneShot"))) {
					return null;
				}
			} catch (NoSuchMethodException ignored) {
			}
			try {
				if (Boolean.TRUE.equals(call(body, "isDuplex"))) {
					return null;
				}
			} catch (NoSuchMethodException ignored) {
			}
			ClassLoader loader = body.getClass().getClassLoader();
			Class<?> bufferClass = Class.forName("okio.Buffer", true, loader);
			Object buffer = bufferClass.getConstructor().newInstance();
			call(body, "writeTo", buffer);
			return (byte[]) call(buffer, "readByteArray");
		} catch (Throwable e) {
			// Reading a body is observational. A custom RequestBody may reject a
			// second write; leave it intact and report an unavailable body.
			return null;
		}
	}

	private static Object overrideRequest(Object request, JSONObject params) throws Throwable {
		Object builder = call(request, "newBuilder");
		if (params.has("url")) {
			call(builder, "url", params.getString("url"));
		}
		if (params.has("headers")) {
			JSONObject headers = params.getJSONObject("headers");
			Object headerBuilder = Class.forName("okhttp3.Headers$Builder", true,
				request.getClass().getClassLoader()).getConstructor().newInstance();
			Iterator<String> names = headers.keys();
			while (names.hasNext()) {
				String name = names.next();
				call(headerBuilder, "add", name, headers.getString(name));
			}
			call(builder, "headers", call(headerBuilder, "build"));
		}
		String method = params.optString("method", (String) call(request, "method"));
		Object body = call(request, "body");
		if (params.has("body")) {
			body = responseBody(request, Base64.decode(params.getString("body"), Base64.DEFAULT), true);
		}
		if (params.has("method") || params.has("body")) {
			call(builder, "method", method, body);
		}
		return call(builder, "build");
	}

	private static Object syntheticResponse(Object request, JSONObject params) throws Throwable {
		ClassLoader loader = request.getClass().getClassLoader();
		Object builder = Class.forName("okhttp3.Response$Builder", true, loader)
			.getConstructor().newInstance();
		call(builder, "request", request);
		Class<?> protocolClass = Class.forName("okhttp3.Protocol", true, loader);
		@SuppressWarnings({"unchecked", "rawtypes"})
		Object http11 = Enum.valueOf((Class<? extends Enum>) protocolClass.asSubclass(Enum.class), "HTTP_1_1");
		call(builder, "protocol", http11);
		int status = params.getInt("status");
		call(builder, "code", status);
		call(builder, "message", "mobilecli synthetic response");
		JSONObject headers = params.optJSONObject("headers");
		if (headers != null) {
			Iterator<String> names = headers.keys();
			while (names.hasNext()) {
				String name = names.next();
				call(builder, "addHeader", name, headers.getString(name));
			}
		}
		byte[] bytes = params.has("body") ? Base64.decode(params.getString("body"), Base64.DEFAULT) : new byte[0];
		call(builder, "body", responseBody(request, bytes, false));
		return call(builder, "build");
	}

	private static Object responseBody(Object request, byte[] bytes, boolean requestBody) throws Throwable {
		ClassLoader loader = request.getClass().getClassLoader();
		Class<?> bodyClass = Class.forName(requestBody ? "okhttp3.RequestBody" : "okhttp3.ResponseBody", true, loader);
		for (Method method : bodyClass.getMethods()) {
			if (!method.getName().equals("create") || !java.lang.reflect.Modifier.isStatic(method.getModifiers())) {
				continue;
			}
			Class<?>[] types = method.getParameterTypes();
			if (types.length != 2) {
				continue;
			}
			if (types[0] == byte[].class && types[1].getName().equals("okhttp3.MediaType")) {
				return invoke(method, null, bytes, null);
			}
			if (types[1] == byte[].class && types[0].getName().equals("okhttp3.MediaType")) {
				return invoke(method, null, null, bytes);
			}
		}
		throw new IOException("unsupported OkHttp body factory");
	}

	private static Object call(Object target, String name, Object... args) throws Throwable {
		for (Method method : target.getClass().getMethods()) {
			if (!method.getName().equals(name) || method.getParameterTypes().length != args.length) {
				continue;
			}
			Class<?>[] types = method.getParameterTypes();
			boolean fits = true;
			for (int i = 0; i < args.length; i++) {
				if (args[i] != null && !boxed(types[i]).isInstance(args[i])) {
					fits = false;
					break;
				}
			}
			if (fits) {
				return invoke(method, target, args);
			}
		}
		throw new NoSuchMethodException(target.getClass().getName() + "." + name);
	}

	private static Class<?> boxed(Class<?> type) {
		if (type == int.class) {
			return Integer.class;
		}
		if (type == long.class) {
			return Long.class;
		}
		if (type == boolean.class) {
			return Boolean.class;
		}
		return type;
	}

	private static Object invoke(Method method, Object target, Object... args) throws Throwable {
		try {
			return method.invoke(target, args);
		} catch (InvocationTargetException e) {
			throw e.getCause();
		}
	}
}
