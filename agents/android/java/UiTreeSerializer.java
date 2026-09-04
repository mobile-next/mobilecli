package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.graphics.Rect;
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
	static JSONObject dump(UiAutomation automation, long waitUntilIdle) throws Exception {
		if (waitUntilIdle > 0) {
			// Best-effort settle: waitForIdle throws when the UI never goes idle
			// (animations, video, spinners). Dump the current state regardless.
			try {
				automation.waitForIdle(IDLE_WINDOW_MS, waitUntilIdle);
			} catch (TimeoutException e) {
				Log.w(TAG, "UI not idle within " + waitUntilIdle + "ms; dumping current state");
			}
		}

		List<AccessibilityNodeInfo> roots = new ArrayList<>();
		for (AccessibilityWindowInfo window : automation.getWindows()) {
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
