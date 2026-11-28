package com.mobilenext.mobilecli;

/**
 * Picks the encoder size for a capture: the requested size when the encoder
 * takes it, otherwise the largest size below it, with the same aspect ratio,
 * that the encoder takes. Like screenrecord, a display larger than the encoder
 * supports is recorded smaller instead of not at all.
 */
final class EncoderSize {

	interface Check {
		boolean isSupported(int width, int height);
	}

	// sizes are tried from 100% down to 1% of the request, in 1% steps
	private static final int STEPS = 100;

	private EncoderSize() {
	}

	/** {width, height} aligned to the encoder, or null when no size fits. */
	static int[] fit(int width, int height, int widthAlign, int heightAlign, Check check) {
		for (int step = STEPS; step > 0; step--) {
			int w = alignDown((int) ((long) width * step / STEPS), widthAlign);
			int h = alignDown((int) ((long) height * step / STEPS), heightAlign);
			if (w > 0 && h > 0 && check.isSupported(w, h)) {
				return new int[] {w, h};
			}
		}
		return null;
	}

	private static int alignDown(int value, int align) {
		return (value / align) * align;
	}
}
