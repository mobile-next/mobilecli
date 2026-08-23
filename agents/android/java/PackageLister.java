package com.mobilenext.mobilecli;

import android.content.pm.ApplicationInfo;
import android.content.pm.PackageInfo;
import android.content.res.AssetManager;
import android.content.res.Configuration;
import android.content.res.Resources;
import android.os.IBinder;
import android.util.DisplayMetrics;

import org.json.JSONArray;
import org.json.JSONObject;

import java.lang.reflect.Method;
import java.util.List;

/**
 * Standalone entry point for listing installed packages via app_process
 * (port of devicekit-android PackageLister.kt).
 *
 * Usage:
 *   adb shell CLASSPATH=/data/local/tmp/mobilecli.dex app_process / com.mobilenext.mobilecli.PackageLister
 *
 * Prints a JSON array of {packageName, appName, version, versionCode} to stdout.
 */
public class PackageLister {

	public static void main(String[] args) {
		try {
			System.out.println(listPackages().toString());
		} catch (Exception e) {
			System.err.println("Error: " + e.getMessage());
			e.printStackTrace(System.err);
			System.exit(1);
		}
	}

	@SuppressWarnings("unchecked")
	static JSONArray listPackages() throws Exception {
		// IPackageManager via ServiceManager (hidden API); app_process runs as shell, no Context available
		Class<?> serviceManager = Class.forName("android.os.ServiceManager");
		IBinder binder = (IBinder) serviceManager.getMethod("getService", String.class).invoke(null, "package");
		Class<?> ipmStub = Class.forName("android.content.pm.IPackageManager$Stub");
		Object ipm = ipmStub.getMethod("asInterface", IBinder.class).invoke(null, binder);

		Object parceledList = getInstalledPackages(ipm);
		List<PackageInfo> packages = (List<PackageInfo>) parceledList.getClass().getMethod("getList").invoke(parceledList);

		JSONArray result = new JSONArray();
		for (PackageInfo pkg : packages) {
			JSONObject entry = new JSONObject();
			entry.put("packageName", pkg.packageName);
			entry.put("appName", displayName(pkg));
			entry.put("version", pkg.versionName == null ? "" : pkg.versionName);
			entry.put("versionCode", versionCode(pkg));
			result.put(entry);
		}
		return result;
	}

	// flags became long in Android 13 (API 33); earlier releases take int
	private static Object getInstalledPackages(Object ipm) throws Exception {
		final int userId = 0;
		try {
			Method m = ipm.getClass().getMethod("getInstalledPackages", long.class, int.class);
			return m.invoke(ipm, 0L, userId);
		} catch (NoSuchMethodException e) {
			Method m = ipm.getClass().getMethod("getInstalledPackages", int.class, int.class);
			return m.invoke(ipm, 0, userId);
		}
	}

	// getLongVersionCode is API 28+; fall back to the int field on 26/27
	@SuppressWarnings("deprecation")
	private static long versionCode(PackageInfo pkg) {
		try {
			return pkg.getLongVersionCode();
		} catch (NoSuchMethodError e) {
			return pkg.versionCode;
		}
	}

	private static String displayName(PackageInfo pkg) {
		ApplicationInfo appInfo = pkg.applicationInfo;
		if (appInfo == null) {
			return pkg.packageName;
		}
		if (appInfo.nonLocalizedLabel != null) {
			return appInfo.nonLocalizedLabel.toString();
		}
		if (appInfo.labelRes != 0 && appInfo.sourceDir != null) {
			try {
				AssetManager assets = AssetManager.class.getDeclaredConstructor().newInstance();
				AssetManager.class.getMethod("addAssetPath", String.class).invoke(assets, appInfo.sourceDir);
				Resources res = new Resources(assets, new DisplayMetrics(), new Configuration());
				String label = res.getString(appInfo.labelRes);
				if (!label.isEmpty()) {
					return label;
				}
			} catch (Exception ignored) {
			}
		}
		return pkg.packageName;
	}
}
