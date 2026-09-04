package com.mobilenext.mobilecli;

import android.hardware.display.DisplayManager;
import android.hardware.display.VirtualDisplay;
import android.os.IBinder;
import android.util.Log;
import android.view.Display;
import android.view.Surface;

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

	/** Mirrors the default display into surface without MediaProjection; null on failure. */
	static VirtualDisplay createVirtualDisplay(String name, int width, int height, int dpi, Surface surface) {
		try {
			// hidden static DisplayManager.createVirtualDisplay(String, int, int, int, Surface)
			return (VirtualDisplay) DisplayManager.class
					.getMethod("createVirtualDisplay", String.class, int.class, int.class, int.class, Surface.class)
					.invoke(null, name, width, height, dpi, surface);
		} catch (Exception e) {
			Log.e(TAG, "Failed to create virtual display", e);
			return null;
		}
	}
}
