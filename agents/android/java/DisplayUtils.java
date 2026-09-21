package com.mobilenext.mobilecli;

import android.graphics.Rect;
import android.hardware.display.DisplayManager;
import android.hardware.display.VirtualDisplay;
import android.os.IBinder;
import android.util.Log;
import android.view.Display;
import android.view.Surface;

import java.lang.reflect.Method;

/** Display info and virtual display creation via hidden APIs (no Context needed). */
class DisplayUtils {

	private static final String TAG = "DisplayUtils";

	static class DisplayInfo {
		final int width, height, dpi, rotation;

		DisplayInfo(int width, int height, int dpi, int rotation) {
			this.width = width;
			this.height = height;
			this.dpi = dpi;
			this.rotation = rotation;
		}
	}

	private static Object displayManager() throws Exception {
		Class<?> serviceManager = Class.forName("android.os.ServiceManager");
		IBinder binder = (IBinder) serviceManager.getMethod("getService", String.class).invoke(null, "display");
		Class<?> stub = Class.forName("android.hardware.display.IDisplayManager$Stub");
		return stub.getMethod("asInterface", IBinder.class).invoke(null, binder);
	}

	/** Primary display geometry; falls back to 1080x1920@320 if reflection fails. */
	static DisplayInfo getDisplayInfo() {
		try {
			Object dm = displayManager();
			Object info = dm.getClass().getMethod("getDisplayInfo", int.class).invoke(dm, Display.DEFAULT_DISPLAY);
			if (info == null) throw new IllegalStateException("DisplayInfo is null");
			Class<?> c = info.getClass();
			return new DisplayInfo(
					c.getField("logicalWidth").getInt(info),
					c.getField("logicalHeight").getInt(info),
					c.getField("logicalDensityDpi").getInt(info),
					c.getField("rotation").getInt(info));
		} catch (Exception e) {
			Log.w(TAG, "Failed to get display info via DisplayManager, using fallback", e);
			return new DisplayInfo(1080, 1920, 320, Surface.ROTATION_0);
		}
	}

	/** A running mirror of the default display; release() stops it. */
	interface DisplayMirror {
		void release();

		/** Has the display drawn into the surface once more, even if nothing on screen changed. */
		void refresh();
	}

	/** Mirrors the default display into surface without MediaProjection; null on failure. */
	static DisplayMirror createVirtualDisplay(String name, int width, int height, Surface surface) {
		try {
			// hidden static DisplayManager.createVirtualDisplay(String name, int width,
			// int height, int displayIdToMirror, Surface surface)
			Method create = DisplayManager.class
					.getMethod("createVirtualDisplay", String.class, int.class, int.class, int.class, Surface.class);
			return new DisplayMirror() {
				private VirtualDisplay display = newDisplay();

				private VirtualDisplay newDisplay() throws Exception {
					return (VirtualDisplay) create.invoke(null, name, width, height, Display.DEFAULT_DISPLAY, surface);
				}

				@Override
				public synchronized void release() {
					display.release();
				}

				@Override
				public synchronized void refresh() {
					// a new display is drawn once when it appears; giving the old one its
					// surface again draws nothing (seen on a Pixel 7, Android 14)
					try {
						display.release();
						display = newDisplay();
					} catch (Exception e) {
						Log.w(TAG, "Failed to refresh the virtual display", e);
					}
				}
			};
		} catch (NoSuchMethodException e) {
			// Android 12 and 13 don't have that method (seen on Pixel 6, Galaxy A53, Pixel 7a).
			// They still have SurfaceControl.createDisplay, which Android 14 removed.
			Log.i(TAG, "DisplayManager can't mirror on this Android version, using SurfaceControl");
		} catch (Exception e) {
			Log.e(TAG, "Failed to create virtual display", e);
			return null;
		}

		try {
			return mirrorWithSurfaceControl(name, width, height, surface);
		} catch (Exception e) {
			Log.e(TAG, "Failed to mirror the display with SurfaceControl", e);
			return null;
		}
	}

	private static DisplayMirror mirrorWithSurfaceControl(String name, int width, int height, Surface surface) throws Exception {
		return new DisplayMirror() {
			private IBinder display = createMirrorDisplay(name, width, height, surface);

			@Override
			public synchronized void release() {
				destroyDisplay(display);
			}

			@Override
			public synchronized void refresh() {
				// a new display is composed once when it appears; re-applying the same
				// surface to the old one changes nothing and draws nothing
				try {
					IBinder fresh = createMirrorDisplay(name, width, height, surface);
					destroyDisplay(display);
					display = fresh;
				} catch (Exception e) {
					Log.w(TAG, "Failed to refresh the mirror display", e);
				}
			}
		};
	}

	private static void destroyDisplay(IBinder display) {
		try {
			Class.forName("android.view.SurfaceControl").getMethod("destroyDisplay", IBinder.class).invoke(null, display);
		} catch (Exception e) {
			Log.w(TAG, "Failed to destroy the mirror display", e);
		}
	}

	private static IBinder createMirrorDisplay(String name, int width, int height, Surface surface) throws Exception {
		Object dm = displayManager();
		Object info = dm.getClass().getMethod("getDisplayInfo", int.class).invoke(dm, Display.DEFAULT_DISPLAY);
		Class<?> infoClass = info.getClass();
		Rect source = new Rect(0, 0, infoClass.getField("logicalWidth").getInt(info), infoClass.getField("logicalHeight").getInt(info));
		int layerStack = infoClass.getField("layerStack").getInt(info);

		Class<?> sc = Class.forName("android.view.SurfaceControl");
		boolean secure = false;
		IBinder display = (IBinder) sc.getMethod("createDisplay", String.class, boolean.class).invoke(null, name, secure);

		sc.getMethod("openTransaction").invoke(null);
		try {
			sc.getMethod("setDisplaySurface", IBinder.class, Surface.class).invoke(null, display, surface);
			sc.getMethod("setDisplayProjection", IBinder.class, int.class, Rect.class, Rect.class)
					.invoke(null, display, Surface.ROTATION_0, source, new Rect(0, 0, width, height));
			sc.getMethod("setDisplayLayerStack", IBinder.class, int.class).invoke(null, display, layerStack);
		} finally {
			sc.getMethod("closeTransaction").invoke(null);
		}

		return display;
	}
}
