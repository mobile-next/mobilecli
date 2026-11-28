package com.mobilenext.mobilecli;

/**
 * Picks the encoder size for a capture: the requested size when the encoder
 * takes it, otherwise the widest size below it, with the same aspect ratio,
 * that the encoder takes. Like screenrecord, a display larger than the encoder
 * supports is recorded smaller instead of not at all.
 */
final class EncoderSize {

	interface Check {
		boolean isSupported(int width, int height);
	}

	private EncoderSize() {
	}

	/**
	 * {width, height} aligned to the encoder, or null when no size fits. Every
	 * aligned width from the request down is tried, so the largest size the
	 * encoder takes is never stepped over.
	 */
	static int[] fit(int width, int height, int widthAlign, int heightAlign, Check check) {
		for (int w = alignDown(width, widthAlign); w > 0; w -= widthAlign) {
			int h = alignDown((int) ((long) height * w / width), heightAlign);
			if (h > 0 && check.isSupported(w, h)) {
				return new int[] {w, h};
			}
		}
		return null;
	}

	private static int alignDown(int value, int align) {
		return (value / align) * align;
	}
}
