package com.mobilenext.mobilecli;

import android.content.ClipData;
import android.os.IBinder;

import java.lang.reflect.Method;

/**
 * System clipboard via the IClipboard binder service, so it works from an
 * app_process (shell uid) with no Context. Served by UiDumpServer.
 *
 * IClipboard's methods have gained parameters across Android versions
 * (attributionTag, userId, deviceId), always appended in a stable order.
 * Rather than hardcode a per-API signature, arguments are filled by parameter
 * type and position, which covers API 29 through current.
 */
public class Clipboard {

	// The clipboard service attributes the call to this caller; shell's package.
	private static final String CALLING_PACKAGE = "com.android.shell";
	private static final int USER_ID = 0;
	private static final int DEVICE_ID = 0; // Context.DEVICE_ID_DEFAULT

	static void setText(String text) throws Exception {
		ClipData clip = ClipData.newPlainText("mobilecli", text);
		Method method = findMethod("setPrimaryClip", ClipData.class);
		method.invoke(clipboard(), buildArgs(method, clip));
	}

	static String getText() throws Exception {
		Method method = findMethod("getPrimaryClip");
		ClipData clip = (ClipData) method.invoke(clipboard(), buildArgs(method, null));
		if (clip == null || clip.getItemCount() == 0) {
			return null;
		}
		CharSequence text = clip.getItemAt(0).getText();
		return text == null ? null : text.toString();
	}

	static void clear() throws Exception {
		Method method = findMethod("clearPrimaryClip");
		method.invoke(clipboard(), buildArgs(method, null));
	}

	private static Class<?> clipboardInterface;
	private static Object clipboard;

	private static Class<?> clipboardInterface() throws Exception {
		if (clipboardInterface == null) {
			clipboardInterface = Class.forName("android.content.IClipboard");
		}
		return clipboardInterface;
	}

	// IClipboard via ServiceManager (hidden API); app_process runs as shell, no Context available
	private static Object clipboard() throws Exception {
		if (clipboard == null) {
			Class<?> serviceManager = Class.forName("android.os.ServiceManager");
			IBinder binder = (IBinder) serviceManager.getMethod("getService", String.class).invoke(null, "clipboard");
			if (binder == null) {
				throw new IllegalStateException("clipboard service not available");
			}
			Class<?> stub = Class.forName("android.content.IClipboard$Stub");
			clipboard = stub.getMethod("asInterface", IBinder.class).invoke(null, binder);
			if (clipboard == null) {
				throw new IllegalStateException("could not bind to IClipboard");
			}
		}
		return clipboard;
	}

	// First overload of name whose leading parameters match leading.
	private static Method findMethod(String name, Class<?>... leading) throws Exception {
		for (Method method : clipboardInterface().getMethods()) {
			if (!method.getName().equals(name)) {
				continue;
			}
			Class<?>[] params = method.getParameterTypes();
			boolean matches = params.length >= leading.length;
			for (int i = 0; matches && i < leading.length; i++) {
				matches = params[i] == leading[i];
			}
			if (matches) {
				return method;
			}
		}
		throw new NoSuchMethodException("IClipboard has no matching " + name + " method");
	}

	private static Object[] buildArgs(Method method, ClipData clip) {
		Class<?>[] params = method.getParameterTypes();
		Object[] args = new Object[params.length];
		int stringSeen = 0;
		int intSeen = 0;
		for (int i = 0; i < params.length; i++) {
			if (params[i] == ClipData.class) {
				args[i] = clip;
			} else if (params[i] == String.class) {
				stringSeen++;
				args[i] = (stringSeen == 1) ? CALLING_PACKAGE : null; // callingPackage, then attributionTag
			} else if (params[i] == int.class) {
				intSeen++;
				args[i] = (intSeen == 1) ? USER_ID : DEVICE_ID; // userId, then deviceId
			} else {
				args[i] = null;
			}
		}
		return args;
	}
}
