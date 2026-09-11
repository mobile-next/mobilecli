package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.os.SystemClock;
import android.view.InputDevice;
import android.view.KeyCharacterMap;
import android.view.KeyEvent;

import org.json.JSONArray;
import org.json.JSONObject;

/**
 * Single-finger touches and key presses, served by DeviceServer as
 * device.io.tap / longpress / swipe / button / keys / text. They replace
 * `adb shell input`, which forks a JVM on the device for every call.
 *
 * Touches are expressed as one-finger gestures and replayed by Gestures, so a
 * tap and a pinch travel the same path. Keys are KEYCODE_* names, resolved on
 * the device with KeyEvent.keyCodeFromString.
 */
public class Input {

	// no hold and no travel: down and up with nothing in between, which is what
	// `adb shell input tap` sends
	static JSONObject tap(UiAutomation automation, int x, int y) throws Exception {
		return Gestures.perform(automation, oneFinger(x, y, 0, x, y, 0));
	}

	static JSONObject longPress(UiAutomation automation, int x, int y, int durationMs) throws Exception {
		return Gestures.perform(automation, oneFinger(x, y, durationMs / 1000.0, x, y, 0));
	}

	static JSONObject swipe(UiAutomation automation, int x1, int y1, int x2, int y2, int durationMs) throws Exception {
		return Gestures.perform(automation, oneFinger(x1, y1, 0, x2, y2, durationMs / 1000.0));
	}

	// press at (x1, y1) for holdSeconds, travel to (x2, y2) over moveSeconds, lift
	private static JSONArray oneFinger(int x1, int y1, double holdSeconds, int x2, int y2, double moveSeconds) throws Exception {
		JSONArray actions = new JSONArray();
		actions.put(new JSONObject().put("type", "press").put("x", x1).put("y", y1).put("duration", holdSeconds));
		if (moveSeconds > 0) {
			actions.put(new JSONObject().put("type", "move").put("x", x2).put("y", y2).put("duration", moveSeconds));
		}
		actions.put(new JSONObject().put("type", "release").put("duration", 0));
		return actions;
	}

	/** Presses one key with zero or more modifiers held: modifiers down, key down/up, modifiers up. */
	static JSONObject pressKey(UiAutomation automation, String keycode, JSONArray modifiers) throws Exception {
		int code = resolveKeycode(keycode);
		int count = modifiers == null ? 0 : modifiers.length();
		int[] modifierCodes = new int[count];
		for (int i = 0; i < count; i++) {
			modifierCodes[i] = resolveKeycode(modifiers.getString(i));
		}

		int meta = 0;
		for (int modifier : modifierCodes) {
			meta |= metaFor(modifier);
			inject(automation, KeyEvent.ACTION_DOWN, modifier, meta);
		}
		inject(automation, KeyEvent.ACTION_DOWN, code, meta);
		inject(automation, KeyEvent.ACTION_UP, code, meta);
		for (int i = count - 1; i >= 0; i--) {
			meta &= ~metaFor(modifierCodes[i]);
			inject(automation, KeyEvent.ACTION_UP, modifierCodes[i], meta);
		}
		return ok();
	}

	/**
	 * Types text through the virtual keyboard's character map, the way
	 * `adb shell input text` does. The map only covers what a physical keyboard
	 * can type, so anything else is INVALID_PARAMS and the host pastes instead.
	 */
	static JSONObject typeText(UiAutomation automation, String text) throws Exception {
		KeyCharacterMap map = KeyCharacterMap.load(KeyCharacterMap.VIRTUAL_KEYBOARD);
		KeyEvent[] events = map.getEvents(text.toCharArray());
		if (events == null) {
			throw new RpcException(RpcException.INVALID_PARAMS, "text has characters the keyboard cannot type");
		}
		for (KeyEvent event : events) {
			long now = SystemClock.uptimeMillis();
			KeyEvent timed = KeyEvent.changeTimeRepeat(event, now, 0);
			if (timed.getSource() == InputDevice.SOURCE_UNKNOWN) {
				timed.setSource(InputDevice.SOURCE_KEYBOARD);
			}
			InputInjector.inject(automation, timed, false);
		}
		return ok();
	}

	private static void inject(UiAutomation automation, int action, int keycode, int meta) throws RpcException {
		long now = SystemClock.uptimeMillis();
		KeyEvent event = new KeyEvent(now, now, action, keycode, 0, meta, KeyCharacterMap.VIRTUAL_KEYBOARD, 0, 0,
				InputDevice.SOURCE_KEYBOARD);
		InputInjector.inject(automation, event, action == KeyEvent.ACTION_UP);
	}

	private static int resolveKeycode(String name) throws RpcException {
		int code = KeyEvent.keyCodeFromString(name);
		if (code == KeyEvent.KEYCODE_UNKNOWN) {
			throw new RpcException(RpcException.INVALID_PARAMS, "unknown keycode " + name);
		}
		return code;
	}

	// the meta flags a real keyboard would report while this modifier is down
	private static int metaFor(int modifier) {
		switch (modifier) {
			case KeyEvent.KEYCODE_SHIFT_LEFT: return KeyEvent.META_SHIFT_ON | KeyEvent.META_SHIFT_LEFT_ON;
			case KeyEvent.KEYCODE_SHIFT_RIGHT: return KeyEvent.META_SHIFT_ON | KeyEvent.META_SHIFT_RIGHT_ON;
			case KeyEvent.KEYCODE_CTRL_LEFT: return KeyEvent.META_CTRL_ON | KeyEvent.META_CTRL_LEFT_ON;
			case KeyEvent.KEYCODE_CTRL_RIGHT: return KeyEvent.META_CTRL_ON | KeyEvent.META_CTRL_RIGHT_ON;
			case KeyEvent.KEYCODE_ALT_LEFT: return KeyEvent.META_ALT_ON | KeyEvent.META_ALT_LEFT_ON;
			case KeyEvent.KEYCODE_ALT_RIGHT: return KeyEvent.META_ALT_ON | KeyEvent.META_ALT_RIGHT_ON;
			case KeyEvent.KEYCODE_META_LEFT: return KeyEvent.META_META_ON | KeyEvent.META_META_LEFT_ON;
			case KeyEvent.KEYCODE_META_RIGHT: return KeyEvent.META_META_ON | KeyEvent.META_META_RIGHT_ON;
			case KeyEvent.KEYCODE_FUNCTION: return KeyEvent.META_FUNCTION_ON;
			default: return 0;
		}
	}

	private static JSONObject ok() throws Exception {
		return new JSONObject().put("ok", true);
	}
}
