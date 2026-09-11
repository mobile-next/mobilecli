package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.os.SystemClock;
import android.view.InputDevice;
import android.view.MotionEvent;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

/**
 * Multi-finger touch injection through the server's UiAutomation, the same
 * object uiautomator's own pinch uses. Served by DeviceServer as
 * "device.io.gesture" with the contract devicekit-ios already speaks: a flat
 * list of {type: press|move|release, x, y, duration, button}, where button is
 * the finger index and duration is in seconds.
 *
 * Actions are grouped by finger and every finger's timeline starts at zero,
 * so two fingers travel at the same time; that is what makes a pinch a pinch
 * rather than two swipes. press holds for its duration before the first move,
 * move travels to (x, y) over its duration, release waits its duration at the
 * last point and then lifts. The whole gesture is replayed in real time, one
 * MotionEvent per frame carrying every finger that is currently down.
 */
public class Gestures {

	private static final long FRAME_MS = 16; // ~60 fps, what a real touchscreen reports

	// JsonRpcSocketServer serves one request at a time, and replay sleeps for
	// the whole gesture, so an over-long one would stall every other call
	// (screenshots included) until it ends. Nothing legitimate takes this long.
	private static final long MAX_GESTURE_MS = 30_000;

	private static final String TYPE_PRESS = "press";
	private static final String TYPE_MOVE = "move";
	private static final String TYPE_RELEASE = "release";

	/** One point on a finger's path, at an offset from the gesture start. */
	private static class Keyframe {
		final long atMs;
		final float x, y;

		Keyframe(long atMs, float x, float y) {
			this.atMs = atMs;
			this.x = x;
			this.y = y;
		}
	}

	/** A finger's whole path; it is down from time zero until the last keyframe. */
	private static class Finger {
		final int id;
		final List<Keyframe> path = new ArrayList<>();

		Finger(int id) {
			this.id = id;
		}

		long liftAtMs() {
			return path.get(path.size() - 1).atMs;
		}

		Keyframe positionAt(long atMs) {
			Keyframe previous = path.get(0);
			for (Keyframe next : path) {
				if (atMs <= next.atMs) {
					if (next.atMs == previous.atMs) return next;
					float fraction = (float) (atMs - previous.atMs) / (next.atMs - previous.atMs);
					return new Keyframe(atMs, previous.x + (next.x - previous.x) * fraction,
							previous.y + (next.y - previous.y) * fraction);
				}
				previous = next;
			}
			return previous;
		}
	}

	static JSONObject perform(UiAutomation automation, JSONArray actions) throws Exception {
		List<Finger> fingers = parseFingers(actions);
		long endMs = 0;
		for (Finger finger : fingers) endMs = Math.max(endMs, finger.liftAtMs());
		if (endMs > MAX_GESTURE_MS) {
			throw new RpcException(RpcException.INVALID_PARAMS,
					"gesture is " + endMs + "ms long, the limit is " + MAX_GESTURE_MS + "ms");
		}
		replay(automation, fingers);
		return new JSONObject().put("ok", true);
	}

	// Groups actions by button into per-finger paths, validating the same way
	// devicekit-ios does so both platforms reject the same input.
	private static List<Finger> parseFingers(JSONArray actions) throws Exception {
		if (actions == null || actions.length() == 0) {
			throw new RpcException(RpcException.INVALID_PARAMS, "actions array cannot be empty");
		}

		Map<Integer, List<JSONObject>> byFinger = new TreeMap<>();
		for (int i = 0; i < actions.length(); i++) {
			JSONObject action = actions.getJSONObject(i);
			int button = action.optInt("button", 0);
			if (!byFinger.containsKey(button)) byFinger.put(button, new ArrayList<JSONObject>());
			byFinger.get(button).add(action);
		}

		List<Finger> fingers = new ArrayList<>();
		for (Map.Entry<Integer, List<JSONObject>> entry : byFinger.entrySet()) {
			fingers.add(buildFinger(entry.getKey(), entry.getValue()));
		}
		return fingers;
	}

	private static Finger buildFinger(int id, List<JSONObject> actions) throws Exception {
		Finger finger = new Finger(id);
		long atMs = 0;
		boolean released = false;

		for (int i = 0; i < actions.size(); i++) {
			JSONObject action = actions.get(i);
			String type = action.optString("type", "");
			float x = (float) action.optDouble("x", 0);
			float y = (float) action.optDouble("y", 0);
			long durationMs = Math.round(action.optDouble("duration", 0) * 1000);

			if (x < 0 || y < 0) {
				throw new RpcException(RpcException.INVALID_PARAMS,
						"finger " + id + " has negative coordinates at index " + i);
			}
			if (durationMs < 0) {
				throw new RpcException(RpcException.INVALID_PARAMS,
						"finger " + id + " has negative duration at index " + i);
			}
			if (released) {
				throw new RpcException(RpcException.INVALID_PARAMS,
						"finger " + id + " has actions after 'release' at index " + i);
			}

			switch (type) {
				case TYPE_PRESS:
					if (i != 0) {
						throw new RpcException(RpcException.INVALID_PARAMS,
								"finger " + id + " has 'press' after its first action at index " + i);
					}
					finger.path.add(new Keyframe(0, x, y));
					atMs += durationMs;
					finger.path.add(new Keyframe(atMs, x, y));
					break;
				case TYPE_MOVE:
					if (i == 0) {
						throw new RpcException(RpcException.INVALID_PARAMS,
								"finger " + id + " must start with 'press', got 'move'");
					}
					atMs += durationMs;
					finger.path.add(new Keyframe(atMs, x, y));
					break;
				case TYPE_RELEASE:
					if (i == 0) {
						throw new RpcException(RpcException.INVALID_PARAMS,
								"finger " + id + " must start with 'press', got 'release'");
					}
					Keyframe last = finger.path.get(finger.path.size() - 1);
					atMs += durationMs;
					finger.path.add(new Keyframe(atMs, last.x, last.y));
					released = true;
					break;
				default:
					throw new RpcException(RpcException.INVALID_PARAMS,
							"finger " + id + " has unknown action type '" + type + "' at index " + i);
			}
		}

		if (!released) {
			throw new RpcException(RpcException.INVALID_PARAMS, "finger " + id + " must end with 'release'");
		}
		return finger;
	}

	// Presses every finger at time zero, then walks the timeline a frame at a
	// time: one ACTION_MOVE for all fingers still down, then a lift for each
	// finger whose path has ended, until the last one is up.
	private static void replay(UiAutomation automation, List<Finger> fingers) {
		long endMs = 0;
		for (Finger finger : fingers) endMs = Math.max(endMs, finger.liftAtMs());

		long downTime = SystemClock.uptimeMillis();
		List<Finger> down = new ArrayList<>();
		for (Finger finger : fingers) {
			down.add(finger);
			int action = down.size() == 1
					? MotionEvent.ACTION_DOWN
					: MotionEvent.ACTION_POINTER_DOWN | ((down.size() - 1) << MotionEvent.ACTION_POINTER_INDEX_SHIFT);
			inject(automation, downTime, downTime, action, down, 0, false);
		}

		for (long atMs = Math.min(FRAME_MS, endMs); !down.isEmpty(); atMs = nextFrame(atMs, downTime, endMs)) {
			sleepUntil(downTime + atMs);
			long eventTime = SystemClock.uptimeMillis();

			inject(automation, downTime, eventTime, MotionEvent.ACTION_MOVE, down, atMs, false);

			for (int i = 0; i < down.size(); ) {
				Finger finger = down.get(i);
				if (finger.liftAtMs() > atMs) {
					i++;
					continue;
				}
				int action = down.size() == 1
						? MotionEvent.ACTION_UP
						: MotionEvent.ACTION_POINTER_UP | (i << MotionEvent.ACTION_POINTER_INDEX_SHIFT);
				inject(automation, downTime, eventTime, action, down, atMs, down.size() == 1);
				down.remove(i);
			}
		}
	}

	// Only the event that lifts the last finger is injected synchronously: waiting
	// for the app to finish handling every intermediate event costs a frame each,
	// which on a tap is most of the call. Waiting for the final one still means the
	// gesture has landed by the time the call returns.
	private static void inject(UiAutomation automation, long downTime, long eventTime, int action,
			List<Finger> pointers, long atMs, boolean sync) {
		int count = pointers.size();
		MotionEvent.PointerProperties[] properties = new MotionEvent.PointerProperties[count];
		MotionEvent.PointerCoords[] coords = new MotionEvent.PointerCoords[count];
		for (int i = 0; i < count; i++) {
			Finger finger = pointers.get(i);
			Keyframe at = finger.positionAt(Math.min(atMs, finger.liftAtMs()));

			properties[i] = new MotionEvent.PointerProperties();
			properties[i].id = finger.id;
			properties[i].toolType = MotionEvent.TOOL_TYPE_FINGER;

			coords[i] = new MotionEvent.PointerCoords();
			coords[i].x = at.x;
			coords[i].y = at.y;
			coords[i].pressure = 1;
			coords[i].size = 1;
		}

		MotionEvent event = MotionEvent.obtain(downTime, eventTime, action, count, properties, coords,
				0, 0, 1, 1, 0, 0, InputDevice.SOURCE_TOUCHSCREEN, 0);
		try {
			InputInjector.inject(automation, event, sync);
		} finally {
			event.recycle();
		}
	}

	// A gesture is replayed in real time, so the clock picks the next frame, not
	// the frame count: injecting is synchronous and can cost more than a frame on
	// a slow device or emulator, and replaying every frame it fell behind on would
	// stretch a 1s swipe into several seconds. Skipping them keeps the gesture the
	// length it was asked for, with fewer intermediate points.
	private static long nextFrame(long atMs, long downTime, long endMs) {
		long elapsed = SystemClock.uptimeMillis() - downTime;
		return Math.min(Math.max(atMs + FRAME_MS, elapsed), endMs);
	}

	private static void sleepUntil(long uptimeMs) {
		long wait = uptimeMs - SystemClock.uptimeMillis();
		if (wait > 0) SystemClock.sleep(wait);
	}
}
