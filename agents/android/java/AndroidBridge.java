package com.mobilenext.mobilecli;

import android.os.Handler;
import android.os.Looper;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.WebView;

import java.lang.reflect.Field;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.stream.IntStream;
import java.util.stream.Stream;

class AndroidBridge {

	static Handler sMainHandler;

	private static String sPackageName;
	private static Field sMViewsField;
	private static Object sWmgInstance;

	static void init() {
		sMainHandler = new Handler(Looper.getMainLooper());
	}

	@SuppressWarnings("unchecked")
	static List<View> getRootViews() throws Exception {
		if (sMViewsField == null) {
			Class<?> wmgClass = Class.forName("android.view.WindowManagerGlobal");
			sMViewsField = wmgClass.getDeclaredField("mViews");
			sMViewsField.setAccessible(true);
			sWmgInstance = wmgClass.getMethod("getInstance").invoke(null);
		}
		return (List<View>) sMViewsField.get(sWmgInstance);
	}

	static String getPackageName() {
		if (sPackageName != null) {
			return sPackageName;
		}
		try {
			byte[] buf = new byte[256];
			java.io.FileInputStream fis = new java.io.FileInputStream("/proc/self/cmdline");
			int len = fis.read(buf);
			fis.close();
			int end = 0;
			while (end < len && buf[end] != 0 && buf[end] != ':') {
				end++;
			}
			sPackageName = new String(buf, 0, end).trim();
			return sPackageName;
		} catch (Exception e) {
			return "unknown";
		}
	}

	static Stream<WebView> streamWebViews(View view) {
		if (view instanceof WebView) {
			return Stream.of((WebView) view);
		} else if (view instanceof ViewGroup) {
			ViewGroup vg = (ViewGroup) view;
			return IntStream.range(0, vg.getChildCount())
				.mapToObj(vg::getChildAt)
				.flatMap(AndroidBridge::streamWebViews);
		}
		return Stream.empty();
	}

	@SuppressWarnings("unchecked")
	static <T> T runOnMainThread(Callable<T> task) throws Exception {
		if (Looper.myLooper() == Looper.getMainLooper()) {
			return task.call();
		}
		Object[] result = {null};
		Exception[] err = {null};
		CountDownLatch latch = new CountDownLatch(1);
		sMainHandler.post(() -> {
			try {
				result[0] = task.call();
			} catch (Exception e) {
				err[0] = e;
			} finally {
				latch.countDown();
			}
		});
		if (!latch.await(5, TimeUnit.SECONDS)) {
			throw new Exception("timed out");
		}
		if (err[0] != null) {
			throw err[0];
		}
		return (T) result[0];
	}

	static String evalJs(WebView wv, String script) throws Exception {
		String[] result = {null};
		CountDownLatch latch = new CountDownLatch(1);
		sMainHandler.post(() -> wv.evaluateJavascript(script, value -> {
			result[0] = value;
			latch.countDown();
		}));
		if (!latch.await(10, TimeUnit.SECONDS)) {
			throw new Exception("evaluateJavascript timed out");
		}
		return result[0];
	}
}
