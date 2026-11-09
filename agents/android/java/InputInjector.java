package com.mobilenext.mobilecli;

import android.app.UiAutomation;
import android.view.InputEvent;

import java.lang.reflect.Method;

/**
 * Injects input events straight into InputManager, the way the `input` command
 * does. UiAutomation.injectInputEvent ends up in the same place but goes
 * through the accessibility connection first, and that extra hop costs tens of
 * milliseconds per event — most of the time a tap or a line of text takes.
 *
 * The singleton moved to InputManagerGlobal in Android 14 and neither class is
 * public API, so it's resolved by reflection once, with UiAutomation as the
 * fallback for platforms that don't expose it.
 */
class InputInjector {

	// InputManager.INJECT_INPUT_EVENT_MODE_*
	private static final int ASYNC = 0;
	private static final int WAIT_FOR_FINISH = 2;

	private static final Object INPUT_MANAGER = resolveInputManager();
	private static final Method INJECT = resolveInject(INPUT_MANAGER);

	/**
	 * Injects one event; sync waits for the app to finish handling it. The
	 * platform refusing an event is an error, not a quiet no-op: a caller told
	 * its tap landed when nothing happened has no way to tell.
	 */
	static void inject(UiAutomation automation, InputEvent event, boolean sync) throws RpcException {
		int mode = sync ? WAIT_FOR_FINISH : ASYNC;
		if (INJECT != null) {
			try {
				if (!(Boolean) INJECT.invoke(INPUT_MANAGER, event, mode)) {
					throw new RpcException(RpcException.INTERNAL_ERROR, "device refused " + event);
				}
				return;
			} catch (RpcException e) {
				throw e;
			} catch (Exception e) {
				// the hidden method went away under us; the public path still works
			}
		}
		if (!automation.injectInputEvent(event, sync)) {
			throw new RpcException(RpcException.INTERNAL_ERROR, "device refused " + event);
		}
	}

	private static Object resolveInputManager() {
		try {
			return Class.forName("android.hardware.input.InputManagerGlobal").getMethod("getInstance").invoke(null);
		} catch (Exception e) {
			try {
				return Class.forName("android.hardware.input.InputManager").getMethod("getInstance").invoke(null);
			} catch (Exception e2) {
				return null;
			}
		}
	}

	private static Method resolveInject(Object inputManager) {
		if (inputManager == null) {
			return null;
		}
		try {
			return inputManager.getClass().getMethod("injectInputEvent", InputEvent.class, int.class);
		} catch (Exception e) {
			return null;
		}
	}
}
