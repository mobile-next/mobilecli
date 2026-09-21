package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.graphics.Rect;
import android.os.Build;
import android.util.Log;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityWindowInfo;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeoutException;

/** Serializes every accessibility window's node tree to the devicekit JSON shape. */
class UiTreeSerializer {

	private static final String TAG = "UiTreeSerializer";

	// The UI must be quiet for this long before waitForIdle returns, bounded by
	// the caller-supplied global timeout. Mirrors UiDevice.waitForIdle semantics.
	private static final long IDLE_WINDOW_MS = 500;

	@SuppressWarnings("deprecation") // recycle() is a no-op on API 33+, still frees pools below
	static JSONObject dump(UiAutomation automation, long waitUntilIdle, boolean full) throws Exception {
		if (waitUntilIdle > 0) {
			// Best-effort settle: waitForIdle throws when the UI never goes idle
			// (animations, video, spinners). Dump the current state regardless.
			try {
				automation.waitForIdle(IDLE_WINDOW_MS, waitUntilIdle);
			} catch (TimeoutException e) {
				Log.w(TAG, "UI not idle within " + waitUntilIdle + "ms; dumping current state");
			}
		}

		clearNodeCache(automation);

		List<AccessibilityNodeInfo> roots = new ArrayList<>();
		for (AccessibilityWindowInfo window : automation.getWindows()) {
			// The soft keyboard is noise for most callers: skip IME windows unless asked for a full dump.
			if (!full && window.getType() == AccessibilityWindowInfo.TYPE_INPUT_METHOD) {
				window.recycle();
				continue;
			}
			AccessibilityNodeInfo root = window.getRoot();
			if (root != null) roots.add(root);
			window.recycle();
		}

		// Windows can be present yet expose null roots (not queryable at the
		// moment of the dump). Fall back to the active window's root.
		if (roots.isEmpty()) {
			AccessibilityNodeInfo active = automation.getRootInActiveWindow();
			if (active != null) roots.add(active);
		}

		try {
			JSONArray array = new JSONArray();
			for (AccessibilityNodeInfo root : roots) {
				array.put(nodeToJson(root, 0));
			}
			return new JSONObject().put("hierarchy", array);
		} finally {
			for (AccessibilityNodeInfo root : roots) root.recycle();
		}
	}

	// This connection is long-lived, so node reads are served from the framework's
	// accessibility cache. A walk that races a content change (a Compose LazyRow
	// swapping placeholders for items) can re-cache the old subtree after the
	// invalidating event, leaving it stale for good. Always walk a fresh tree.
	private static void clearNodeCache(UiAutomation automation) {
		if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
			automation.clearCache();
			return;
		}
		try {
			Class<?> client = Class.forName("android.view.accessibility.AccessibilityInteractionClient");
			Object instance = client.getMethod("getInstance").invoke(null);
			if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
				// Android 13 made the cache per-connection, a release before
				// UiAutomation.clearCache() became public.
				Object connectionId = UiAutomation.class.getMethod("getConnectionId").invoke(automation);
				client.getMethod("clearCache", int.class).invoke(instance, connectionId);
				return;
			}
			client.getMethod("clearCache").invoke(instance);
		} catch (ReflectiveOperationException e) {
			Log.w(TAG, "could not clear the accessibility cache; the dump may be stale", e);
		}
	}

	private static String str(CharSequence cs) {
		return cs == null ? "" : cs.toString();
	}

	@SuppressWarnings("deprecation")
	private static JSONObject nodeToJson(AccessibilityNodeInfo node, int index) throws Exception {
		Rect bounds = new Rect();
		node.getBoundsInScreen(bounds);

		// When a field is empty, getText() returns the hint. Separate the two:
		// empty text for an empty field, and the hint as its own attribute.
		String rawText = str(node.getText());
		String hintText = str(node.getHintText());
		boolean showingHint = node.isShowingHintText();
		String text = showingHint ? "" : rawText;
		String hint = !hintText.isEmpty() ? hintText : (showingHint ? rawText : "");

		JSONObject obj = new JSONObject()
				.put("index", index)
				.put("class", str(node.getClassName()))
				.put("package", str(node.getPackageName()))
				.put("text", text)
				.put("hint", hint)
				.put("content-desc", str(node.getContentDescription()))
				.put("resource-id", node.getViewIdResourceName() == null ? "" : node.getViewIdResourceName())
				.put("checkable", node.isCheckable())
				.put("checked", node.isChecked())
				.put("clickable", node.isClickable())
				.put("enabled", node.isEnabled())
				.put("focusable", node.isFocusable())
				.put("focused", node.isFocused())
				.put("scrollable", node.isScrollable())
				.put("long-clickable", node.isLongClickable())
				.put("password", node.isPassword())
				.put("selected", node.isSelected())
				.put("visible", node.isVisibleToUser())
				.put("rect", new JSONObject()
						.put("x", bounds.left).put("y", bounds.top)
						.put("width", bounds.width()).put("height", bounds.height()));

		JSONArray children = new JSONArray();
		for (int i = 0; i < node.getChildCount(); i++) {
			AccessibilityNodeInfo child = node.getChild(i);
			if (child == null) continue;
			try {
				children.put(nodeToJson(child, i));
			} finally {
				child.recycle();
			}
		}
		if (children.length() > 0) {
			obj.put("children", children);
		}
		return obj;
	}
}
