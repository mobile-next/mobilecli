package com.mobilenext.mobilecli;

import android.content.ClipData;
import android.os.IBinder;
import android.util.Base64;

import java.lang.reflect.Method;

/**
 * Standalone clipboard tool via app_process
 * (port of devicekit-android Clipboard.kt / ClipboardService.kt).
 *
 * Talks to the IClipboard binder service directly, so it needs no installed
 * app or running receiver. Must be run as the shell or root user.
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex app_process / \
 *     com.mobilenext.mobilecli.Clipboard <command>
 *
 *   set <text>            set the clipboard to the given text
 *   set --base64 <b64>    set the clipboard to UTF-8 decoded from base64
 *   get                   print the current clipboard text to stdout
 *   clear                 clear the clipboard
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

	public static void main(String[] args) {
		try {
			String command = args.length > 0 ? args[0] : "";
			if (command.equals("set")) {
				setText(parseSetText(args));
			} else if (command.equals("get")) {
				String text = getText();
				System.out.println(text == null ? "" : text);
			} else if (command.equals("clear")) {
				clear();
			} else {
				System.err.println("Usage: Clipboard set <text> | set --base64 <b64> | get | clear");
				System.exit(2);
			}
			System.exit(0);
		} catch (Exception e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

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

	private static String parseSetText(String[] args) {
		if (args.length > 1 && args[1].equals("--base64")) {
			if (args.length < 3) {
				throw new IllegalArgumentException("missing base64 argument");
			}
			return new String(Base64.decode(args[2], Base64.DEFAULT), java.nio.charset.StandardCharsets.UTF_8);
		}
		if (args.length < 2) {
			throw new IllegalArgumentException("missing text argument");
		}
		return args[1];
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
