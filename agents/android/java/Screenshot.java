package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.graphics.Bitmap;
import android.os.HandlerThread;
import android.os.Looper;

import java.io.BufferedOutputStream;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.OutputStream;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;

/**
 * Standalone screenshot tool via app_process.
 *
 * Captures the default display through UiAutomation (the same path the
 * platform's own "uiautomator dump" shell tool uses from app_process),
 * optionally downscales, and writes PNG or JPEG bytes to stdout. Doing the
 * encode on-device means a JPEG crosses adb instead of screencap's PNG.
 * Must be run as the shell or root user.
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex app_process / \
 *     com.mobilenext.mobilecli.Screenshot [--format png|jpeg] [--quality 1-100] \
 *     [--scale 0.0-1.0] [--max-size N] [--clip x,y,w,h --screen-width N]
 *
 * --max-size caps max(width, height) at N pixels keeping aspect ratio and
 * takes precedence over --scale. Neither ever upscales.
 * --clip crops to the given rect (in screen points) before scaling;
 * --screen-width (screen width in points) is required with it to map
 * points to pixels of the captured bitmap.
 */
public class Screenshot {

	public static void main(String[] args) {
		try {
			String format = "png";
			int quality = 90;
			double scale = 1.0;
			int maxSize = 0;
			String outPath = null;
			int[] clip = null;
			int screenWidth = 0;

			for (int i = 0; i + 1 < args.length; i += 2) {
				String flag = args[i];
				String value = args[i + 1];
				if (flag.equals("--format")) {
					format = value;
				} else if (flag.equals("--quality")) {
					quality = Integer.parseInt(value);
				} else if (flag.equals("--scale")) {
					scale = Double.parseDouble(value);
				} else if (flag.equals("--max-size")) {
					maxSize = Integer.parseInt(value);
				} else if (flag.equals("--clip")) {
					clip = parseClip(value);
				} else if (flag.equals("--screen-width")) {
					screenWidth = Integer.parseInt(value);
				} else if (flag.equals("--out")) {
					outPath = value;
				} else {
					throw new IllegalArgumentException("unknown flag: " + flag);
				}
			}

			exemptHiddenApis();
			Bitmap bitmap = takeScreenshot();
			if (clip != null) {
				bitmap = clipBitmap(bitmap, clip, screenWidth);
			}
			bitmap = resize(bitmap, scale, maxSize);

			Bitmap.CompressFormat compressFormat = format.equals("jpeg")
					? Bitmap.CompressFormat.JPEG
					: Bitmap.CompressFormat.PNG;
			// --out avoids stdout, which other libraries (e.g. the emulator's
			// GL layer) pollute with log text that would corrupt the stream
			OutputStream out = new BufferedOutputStream(outPath != null
					? new FileOutputStream(outPath)
					: new FileOutputStream(FileDescriptor.out));
			if (!bitmap.compress(compressFormat, quality, out)) {
				throw new IllegalStateException("Bitmap.compress failed");
			}
			out.flush();
			System.exit(0);
		} catch (Throwable e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

	// Lift hidden-API restrictions for this process so the UiAutomation
	// plumbing below is reachable. Double reflection bypasses the check on
	// VMRuntime itself.
	private static void exemptHiddenApis() throws Exception {
		Method getDeclaredMethod = Class.class.getDeclaredMethod("getDeclaredMethod", String.class, Class[].class);
		Class<?> vmRuntime = Class.forName("dalvik.system.VMRuntime");
		Method getRuntime = (Method) getDeclaredMethod.invoke(vmRuntime, "getRuntime", null);
		Method setExemptions = (Method) getDeclaredMethod.invoke(vmRuntime, "setHiddenApiExemptions", new Class[]{String[].class});
		setExemptions.invoke(getRuntime.invoke(null), new Object[]{new String[]{"L"}});
	}

	// Build a UiAutomation the way UiAutomatorShellWrapper does: a
	// UiAutomationConnection over a background Looper, then connect().
	// The constructor and connect() signatures vary across API levels, so
	// both are matched by parameter types.
	private static Bitmap takeScreenshot() throws Exception {
		// the accessibility plumbing behind UiAutomation expects a main
		// looper, which app_process does not prepare (uiautomator's own
		// Launcher does the same)
		if (Looper.getMainLooper() == null) {
			Looper.prepareMainLooper();
		}

		HandlerThread thread = new HandlerThread("mobilecli-screenshot");
		thread.start();

		Object connection = Class.forName("android.app.UiAutomationConnection")
				.getDeclaredConstructor().newInstance();

		UiAutomation automation = newUiAutomation(thread.getLooper(), connection);
		connect(automation);
		try {
			Bitmap bitmap = automation.takeScreenshot();
			if (bitmap == null) {
				throw new IllegalStateException("takeScreenshot returned null");
			}
			// Hardware bitmaps (API 31+) cannot be scaled; copy to software.
			if (bitmap.getConfig() == Bitmap.Config.HARDWARE) {
				bitmap = bitmap.copy(Bitmap.Config.ARGB_8888, false);
			}
			return bitmap;
		} finally {
			try {
				UiAutomation.class.getDeclaredMethod("disconnect").invoke(automation);
			} catch (Throwable ignored) {
				// best effort; the process exits right after
			}
		}
	}

	// (Looper, IUiAutomationConnection) on older APIs; a Context and an int
	// displayId were prepended in later ones. Pick whichever constructor
	// exists and fill arguments by type: Context stays null (unused for
	// screenshots), int becomes 0 (Display.DEFAULT_DISPLAY).
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

	private static int[] parseClip(String value) {
		String[] parts = value.split(",", -1);
		if (parts.length != 4) {
			throw new IllegalArgumentException("invalid --clip, expected x,y,width,height");
		}
		int[] clip = new int[4];
		for (int i = 0; i < 4; i++) {
			clip[i] = Integer.parseInt(parts[i]);
		}
		return clip;
	}

	// Crops to clip {x, y, width, height} given in screen points, mapped to
	// bitmap pixels via screenWidthPoints. Rects partially outside the bitmap
	// are clamped (UI hierarchy bounds can overhang the screen); rects fully
	// outside are an error. Mirrors cropRectInPixels in utils/image.go.
	private static Bitmap clipBitmap(Bitmap bitmap, int[] clip, int screenWidthPoints) {
		if (clip[2] <= 0 || clip[3] <= 0) {
			throw new IllegalArgumentException("clip width and height must be positive");
		}
		if (screenWidthPoints <= 0) {
			throw new IllegalArgumentException("--screen-width is required with --clip");
		}

		double factor = (double) bitmap.getWidth() / screenWidthPoints;
		int left = Math.max(0, (int) Math.round(clip[0] * factor));
		int top = Math.max(0, (int) Math.round(clip[1] * factor));
		int right = Math.min(bitmap.getWidth(), (int) Math.round((clip[0] + clip[2]) * factor));
		int bottom = Math.min(bitmap.getHeight(), (int) Math.round((clip[1] + clip[3]) * factor));

		if (right <= left || bottom <= top) {
			throw new IllegalArgumentException("clip rect is outside the screenshot bounds");
		}
		return Bitmap.createBitmap(bitmap, left, top, right - left, bottom - top);
	}

	// maxSize caps max(width, height) and wins over scale; never upscales.
	private static Bitmap resize(Bitmap bitmap, double scale, int maxSize) {
		int width = bitmap.getWidth();
		int height = bitmap.getHeight();
		int largest = Math.max(width, height);

		double factor;
		if (maxSize > 0) {
			factor = maxSize >= largest ? 1.0 : (double) maxSize / largest;
		} else {
			factor = scale;
		}

		if (factor <= 0 || factor >= 1.0) {
			return bitmap;
		}

		int newWidth = Math.max(1, (int) Math.round(width * factor));
		int newHeight = Math.max(1, (int) Math.round(height * factor));

		// bilinear filtering only samples 4 pixels, so a single large
		// downscale undersamples and looks like nearest neighbor; halve
		// stepwise (mipmap-style) until within 2x of the target first
		while (bitmap.getWidth() >= newWidth * 2 && bitmap.getHeight() >= newHeight * 2) {
			bitmap = Bitmap.createScaledBitmap(bitmap, bitmap.getWidth() / 2, bitmap.getHeight() / 2, true);
		}
		return Bitmap.createScaledBitmap(bitmap, newWidth, newHeight, true);
	}
}
