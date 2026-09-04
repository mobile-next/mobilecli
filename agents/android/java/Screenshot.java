package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.graphics.Bitmap;

import java.io.ByteArrayOutputStream;

/**
 * Captures the default display through the server's UiAutomation (the same
 * path the platform's own "uiautomator" shell tool uses), optionally crops and
 * downscales, and encodes PNG or JPEG on-device so a small JPEG crosses adb
 * instead of screencap's PNG. Served by DeviceServer as "device.screenshot".
 *
 * maxSize caps max(width, height) keeping aspect ratio and takes precedence
 * over scale. Neither ever upscales. clip {x, y, width, height} is in screen
 * points; screenWidthPoints maps points to pixels of the captured bitmap.
 */
public class Screenshot {

	static byte[] capture(UiAutomation automation, String format, int quality, double scale, int maxSize,
			int[] clip, int screenWidthPoints) throws Exception {
		Bitmap bitmap = automation.takeScreenshot();
		if (bitmap == null) {
			throw new IllegalStateException("takeScreenshot returned null");
		}
		// Hardware bitmaps (API 31+) cannot be scaled; copy to software.
		if (bitmap.getConfig() == Bitmap.Config.HARDWARE) {
			bitmap = replace(bitmap, bitmap.copy(Bitmap.Config.ARGB_8888, false));
		}
		if (clip != null) {
			bitmap = replace(bitmap, clipBitmap(bitmap, clip, screenWidthPoints));
		}
		bitmap = replace(bitmap, resize(bitmap, scale, maxSize));

		Bitmap.CompressFormat compressFormat = format.equals("jpeg")
				? Bitmap.CompressFormat.JPEG
				: Bitmap.CompressFormat.PNG;
		ByteArrayOutputStream out = new ByteArrayOutputStream();
		try {
			if (!bitmap.compress(compressFormat, quality, out)) {
				throw new IllegalStateException("Bitmap.compress failed");
			}
		} finally {
			bitmap.recycle();
		}
		return out.toByteArray();
	}

	// The server process lives on between screenshots, so full-screen bitmaps
	// superseded along the way are recycled instead of waiting for GC.
	private static Bitmap replace(Bitmap old, Bitmap next) {
		if (next != old) {
			old.recycle();
		}
		return next;
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
		Bitmap current = bitmap;
		while (current.getWidth() >= newWidth * 2 && current.getHeight() >= newHeight * 2) {
			Bitmap halved = Bitmap.createScaledBitmap(current, current.getWidth() / 2, current.getHeight() / 2, true);
			if (current != bitmap) {
				current.recycle(); // the caller still owns the original
			}
			current = halved;
		}
		Bitmap result = Bitmap.createScaledBitmap(current, newWidth, newHeight, true);
		if (current != bitmap && current != result) {
			current.recycle();
		}
		return result;
	}
}
