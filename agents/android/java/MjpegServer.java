package com.mobilenext.mobilecli;

import android.graphics.Bitmap;
import android.graphics.PixelFormat;
import android.hardware.display.VirtualDisplay;
import android.media.Image;
import android.media.ImageReader;
import android.os.Handler;
import android.os.HandlerThread;
import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.PrintStream;
import java.nio.ByteBuffer;
import java.util.concurrent.CountDownLatch;

/**
 * Streams the screen as multipart MJPEG frames to stdout via app_process.
 *
 * Usage:
 *   adb exec-out CLASSPATH=/data/local/tmp/mobilecli.dex app_process / \
 *     com.mobilenext.mobilecli.MjpegServer [--quality 1-100] [--scale 0.1-2.0] [--fps 1-60]
 */
public class MjpegServer {

	private static final String TAG = "MjpegServer";
	private static final String BOUNDARY = "BoundaryString";

	private final int quality;
	private final float scale;
	private final CountDownLatch shutdown = new CountDownLatch(1);

	MjpegServer(int quality, float scale) {
		this.quality = quality;
		this.scale = scale;
	}

	public static void main(String[] args) {
		int quality = 80;
		float scale = 1.0f;
		// fps is accepted for CLI compatibility; ImageReader delivers frames as
		// the display produces them, so it is not rate-limited here.
		for (int i = 0; i + 1 < args.length; i += 2) {
			if (args[i].equals("--quality")) {
				quality = Math.max(1, Math.min(100, Integer.parseInt(args[i + 1])));
			} else if (args[i].equals("--scale")) {
				scale = Math.max(0.1f, Math.min(2.0f, Float.parseFloat(args[i + 1])));
			}
		}
		try {
			new MjpegServer(quality, scale).stream();
		} catch (Exception e) {
			Log.e(TAG, "Failed to start MJPEG stream", e);
			System.err.println("Error: " + e.getMessage());
			System.exit(1);
		}
	}

	private void stream() throws InterruptedException {
		Runtime.getRuntime().addShutdownHook(new Thread(shutdown::countDown));

		DisplayUtils.DisplayInfo display = DisplayUtils.getDisplayInfo();
		HandlerThread thread = new HandlerThread("ScreenCapture");
		thread.start();

		ImageReader reader = ImageReader.newInstance(display.width, display.height, PixelFormat.RGBA_8888, 2);
		PrintStream out = System.out;
		reader.setOnImageAvailableListener(r -> {
			Image image = null;
			try {
				image = r.acquireLatestImage();
				if (image != null) {
					writeFrame(out, toJpeg(image, quality, scale));
				}
			} catch (Exception e) {
				Log.e(TAG, "Error processing frame", e);
			} finally {
				if (image != null) image.close();
			}
		}, new Handler(thread.getLooper()));

		VirtualDisplay virtualDisplay = DisplayUtils.createVirtualDisplay(
				"mjpeg.screen.capture", display.width, display.height, display.dpi, reader.getSurface());
		if (virtualDisplay == null) {
			System.err.println("Error: Failed to create virtual display");
			System.exit(1);
		}

		try {
			shutdown.await();
		} finally {
			virtualDisplay.release();
			reader.close();
			thread.quitSafely();
		}
	}

	private void writeFrame(PrintStream out, byte[] jpeg) {
		try {
			out.print("--" + BOUNDARY + "\r\nContent-type: image/jpeg\r\nContent-Length: " + jpeg.length + "\r\n\r\n");
			out.write(jpeg);
			out.print("\r\n");
			out.flush();
		} catch (IOException e) {
			// pipe broken - client disconnected
			shutdown.countDown();
		}
	}

	static byte[] toJpeg(Image image, int quality, float scale) {
		Image.Plane plane = image.getPlanes()[0];
		ByteBuffer buffer = plane.getBuffer();
		int pixelStride = plane.getPixelStride();
		int rowPadding = plane.getRowStride() - pixelStride * image.getWidth();

		Bitmap bitmap = Bitmap.createBitmap(image.getWidth() + rowPadding / pixelStride, image.getHeight(), Bitmap.Config.ARGB_8888);
		bitmap.copyPixelsFromBuffer(buffer);

		Bitmap cropped = rowPadding == 0 ? bitmap : Bitmap.createBitmap(bitmap, 0, 0, image.getWidth(), image.getHeight());
		Bitmap scaled = scale == 1.0f ? cropped
				: Bitmap.createScaledBitmap(cropped, (int) (image.getWidth() * scale), (int) (image.getHeight() * scale), true);

		ByteArrayOutputStream jpeg = new ByteArrayOutputStream();
		scaled.compress(Bitmap.CompressFormat.JPEG, quality, jpeg);

		if (scaled != cropped) scaled.recycle();
		if (cropped != bitmap) cropped.recycle();
		bitmap.recycle();
		return jpeg.toByteArray();
	}
}
