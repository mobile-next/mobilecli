package com.mobilenext.mobilecli;

import android.content.Context;
import android.content.ContextWrapper;
import android.location.Location;
import android.location.LocationManager;
import android.os.IBinder;
import android.os.SystemClock;

import java.lang.reflect.Constructor;

/**
 * Fakes the device location via LocationManager test providers, for real
 * devices where there is no adb-level way to do it. Served by DeviceServer.
 *
 * The test providers only live as long as the hosting process does, so
 * setting a location starts a thread that re-publishes it on a loop; clear()
 * stops it and removes the providers.
 *
 * Requires: adb shell appops set com.android.shell android:mock_location allow
 */
public class MockLocation {

	// providers we publish to. "fused" is what most apps end up reading, "gps"
	// and "network" cover apps talking to LocationManager directly. Not every
	// provider exists on every device, so failures are per-provider.
	private static final String[] PROVIDERS = {
		LocationManager.GPS_PROVIDER,
		LocationManager.NETWORK_PROVIDER,
		"fused",
	};

	private static final String SHELL_PACKAGE = "com.android.shell";
	private static final long PUBLISH_INTERVAL_MS = 1000;

	private static Thread publisher;

	/** Registers the test providers and starts publishing lat/lon until clear(). */
	static synchronized void start(double latitude, double longitude) throws Exception {
		stopPublisher();
		LocationManager locationManager = createLocationManager();
		if (addTestProviders(locationManager) == 0) {
			throw new IllegalStateException("no test provider could be added, is 'appops set " + SHELL_PACKAGE + " android:mock_location allow' in effect?");
		}

		publisher = new Thread(() -> {
			while (!Thread.currentThread().isInterrupted()) {
				for (String provider : PROVIDERS) {
					try {
						locationManager.setTestProviderLocation(provider, newLocation(provider, latitude, longitude));
					} catch (Exception ignored) {
						// provider not registered on this device
					}
				}
				try {
					Thread.sleep(PUBLISH_INTERVAL_MS);
				} catch (InterruptedException e) {
					return;
				}
			}
		}, "mock-location");
		publisher.setDaemon(true);
		publisher.start();
	}

	/** Stops publishing and removes the test providers. */
	static synchronized void clear() throws Exception {
		stopPublisher();
		removeTestProviders(createLocationManager());
	}

	private static void stopPublisher() {
		if (publisher != null) {
			publisher.interrupt();
			publisher = null;
		}
	}

	@SuppressWarnings("deprecation") // Criteria is what LocationManager still expects for test providers
	private static int addTestProviders(LocationManager locationManager) {
		int added = 0;
		for (String provider : PROVIDERS) {
			try {
				try {
					locationManager.removeTestProvider(provider);
				} catch (Exception ignored) {
					// no leftover provider from a previous run
				}
				locationManager.addTestProvider(provider, false, false, false, false, true, true, true,
					android.location.Criteria.POWER_LOW, android.location.Criteria.ACCURACY_FINE);
				locationManager.setTestProviderEnabled(provider, true);
				added++;
			} catch (Exception e) {
				System.err.println("could not add test provider " + provider + ": " + e);
			}
		}
		return added;
	}

	private static void removeTestProviders(LocationManager locationManager) {
		for (String provider : PROVIDERS) {
			try {
				locationManager.removeTestProvider(provider);
			} catch (Exception ignored) {
				// provider was never registered
			}
		}
	}

	private static Location newLocation(String provider, double latitude, double longitude) {
		Location location = new Location(provider);
		location.setLatitude(latitude);
		location.setLongitude(longitude);
		location.setAltitude(0);
		location.setAccuracy(1.0f);
		location.setTime(System.currentTimeMillis());
		location.setElapsedRealtimeNanos(SystemClock.elapsedRealtimeNanos());
		return location;
	}

	/**
	 * Builds a LocationManager without a real Context: app_process runs as shell,
	 * which has no application context, so we hand it the "location" system
	 * service binder directly and a context that identifies as com.android.shell
	 * (the package the mock_location appop is granted to).
	 */
	private static LocationManager createLocationManager() throws Exception {
		Class<?> serviceManager = Class.forName("android.os.ServiceManager");
		IBinder binder = (IBinder) serviceManager.getMethod("getService", String.class).invoke(null, "location");
		if (binder == null) {
			throw new IllegalStateException("location service not available");
		}

		Class<?> stub = Class.forName("android.location.ILocationManager$Stub");
		Object service = stub.getMethod("asInterface", IBinder.class).invoke(null, binder);

		Constructor<LocationManager> constructor = LocationManager.class.getDeclaredConstructor(
			Context.class, Class.forName("android.location.ILocationManager"));
		constructor.setAccessible(true);
		return constructor.newInstance(new ShellContext(), service);
	}

	/**
	 * Minimal Context that only answers the identity questions LocationManager
	 * asks before every binder call. It has no base context, so anything else
	 * would throw - which is fine, nothing else is used.
	 */
	private static class ShellContext extends ContextWrapper {
		ShellContext() {
			super(null);
		}

		@Override
		public String getPackageName() {
			return SHELL_PACKAGE;
		}

		// hidden on the public SDK, so no @Override: it still overrides at runtime
		public String getOpPackageName() {
			return SHELL_PACKAGE;
		}

		@Override
		public String getAttributionTag() {
			return null;
		}
	}
}
