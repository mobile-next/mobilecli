package com.mobilenext.mobilecli;

import android.accessibilityservice.AccessibilityServiceInfo;
import android.app.UiAutomation;
import android.os.HandlerThread;
import android.os.Looper;
import android.view.accessibility.AccessibilityEvent;

import java.lang.reflect.Constructor;
import java.lang.reflect.Method;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/**
 * Builds and connects a UiAutomation without an Instrumentation host, so UI
 * automation can run from a standalone app_process invocation (the same path
 * the platform's own "uiautomator" shell tool uses). Must run as the shell
 * (uid 2000) or root user. Relies on @hide framework members reached via
 * reflection, so signatures are matched by parameter type rather than hardcoded.
 */
class UiAutomationFactory {

	// How long to wait for the first accessibility event after configuring the
	// service, signalling the connection is live before the first query.
	private static final long CONNECT_TIMEOUT_SECONDS = 2;

	/** Fresh, connected UiAutomation on its own looper thread. */
	@SuppressWarnings("deprecation") // prepareMainLooper: app_process has no ActivityThread to do it
	static UiAutomation createAndConnect() throws Exception {
		exemptHiddenApis();

		// the accessibility plumbing behind UiAutomation expects a main
		// looper, which app_process does not prepare
		if (Looper.getMainLooper() == null) {
			Looper.prepareMainLooper();
		}

		HandlerThread thread = new HandlerThread("mobilecli-uiautomation");
		thread.start();

		Object connection = Class.forName("android.app.UiAutomationConnection")
				.getDeclaredConstructor().newInstance();
		UiAutomation automation = newUiAutomation(thread.getLooper(), connection);
		connect(automation);
		return automation;
	}

	/** Subscribes to all events and requests the flags needed to walk every window. */
	static void configureForWindowRetrieval(UiAutomation automation) throws InterruptedException {
		CountDownLatch latch = new CountDownLatch(1);
		automation.setOnAccessibilityEventListener(event -> latch.countDown());
		AccessibilityServiceInfo info = new AccessibilityServiceInfo();
		info.eventTypes = AccessibilityEvent.TYPES_ALL_MASK;
		info.feedbackType = AccessibilityServiceInfo.FEEDBACK_GENERIC;
		info.flags = AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS
				| AccessibilityServiceInfo.FLAG_REPORT_VIEW_IDS
				| AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS;
		automation.setServiceInfo(info);
		latch.await(CONNECT_TIMEOUT_SECONDS, TimeUnit.SECONDS);
		automation.setOnAccessibilityEventListener(null);
	}

	static void disconnect(UiAutomation automation) {
		try {
			UiAutomation.class.getDeclaredMethod("disconnect").invoke(automation);
		} catch (Throwable ignored) {
			// best effort; the process exits right after
		}
	}

	// Lift hidden-API restrictions for this process so the UiAutomation
	// plumbing below is reachable. Double reflection bypasses the check on
	// VMRuntime itself.
	private static void exemptHiddenApis() throws Exception {
		Method getDeclaredMethod = Class.class.getDeclaredMethod("getDeclaredMethod", String.class, Class[].class);
		Class<?> vmRuntime = Class.forName("dalvik.system.VMRuntime");
		Method getRuntime = (Method) getDeclaredMethod.invoke(vmRuntime, "getRuntime", null);
		Method setExemptions = (Method) getDeclaredMethod.invoke(vmRuntime, "setHiddenApiExemptions", new Class<?>[]{String[].class});
		setExemptions.invoke(getRuntime.invoke(null), new Object[]{new String[]{"L"}});
	}

	// (Looper, IUiAutomationConnection) on older APIs; a Context and an int
	// displayId were prepended in later ones. Pick whichever constructor
	// exists and fill arguments by type: Context stays null, int becomes 0
	// (Display.DEFAULT_DISPLAY).
	private static UiAutomation newUiAutomation(Looper looper, Object connection) throws Exception {
		for (Constructor<?> ctor : UiAutomation.class.getDeclaredConstructors()) {
			Class<?>[] params = ctor.getParameterTypes();
			Object[] values = new Object[params.length];
			boolean hasLooper = false;
			boolean hasConnection = false;
			for (int i = 0; i < params.length; i++) {
				if (params[i] == Looper.class) {
					values[i] = looper;
					hasLooper = true;
				} else if (params[i].isInstance(connection)) {
					values[i] = connection;
					hasConnection = true;
				} else if (params[i] == int.class) {
					values[i] = 0;
				}
			}
			if (hasLooper && hasConnection) {
				ctor.setAccessible(true);
				return (UiAutomation) ctor.newInstance(values);
			}
		}
		throw new NoSuchMethodException("no usable UiAutomation constructor");
	}

	// connect() on older APIs, connect(int flags) on newer ones.
	private static void connect(UiAutomation automation) throws Exception {
		try {
			Method connect = UiAutomation.class.getDeclaredMethod("connect");
			connect.invoke(automation);
		} catch (NoSuchMethodException e) {
			Method connect = UiAutomation.class.getDeclaredMethod("connect", int.class);
			connect.invoke(automation, 0);
		}
	}
}
