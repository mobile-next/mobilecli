package com.mobilenext.mobilecli;

import java.io.ByteArrayOutputStream;
import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;

/**
 * Stamps each encoded picture with its capture time, as a user_data_unregistered
 * SEI NAL unit placed right before the picture's first slice. The host's MP4
 * muxer (internal/avc2mp4) times every frame from this SEI and drops pictures
 * without one, so a recording made from this stream gets the video's real
 * duration rather than an assumed frame rate. Decoders ignore the SEI.
 */
final class AvcTimecode {

	// must match mobilenxTimecodeUUID in internal/avc2mp4/seiparser.go
	private static final byte[] UUID = "MOBILENXTIMECODE".getBytes(StandardCharsets.US_ASCII);

	private static final int NAL_TYPE_SLICE = 1;
	private static final int NAL_TYPE_IDR = 5;
	private static final int NAL_TYPE_SEI = 6;
	private static final int SEI_USER_DATA_UNREGISTERED = 5;
	private static final int TIMESTAMP_BYTES = 8;
	private static final int RBSP_STOP_BIT = 0x80;

	private AvcTimecode() {
	}

	/** An annex-b SEI NAL unit, start code included, carrying timestampUs. */
	static byte[] sei(long timestampUs) {
		ByteArrayOutputStream rbsp = new ByteArrayOutputStream();
		rbsp.write(SEI_USER_DATA_UNREGISTERED);
		rbsp.write(UUID.length + TIMESTAMP_BYTES);
		rbsp.write(UUID, 0, UUID.length);
		for (int shift = 56; shift >= 0; shift -= 8) {
			rbsp.write((int) (timestampUs >>> shift) & 0xFF);
		}
		rbsp.write(RBSP_STOP_BIT);

		ByteArrayOutputStream nal = new ByteArrayOutputStream();
		nal.write(0);
		nal.write(0);
		nal.write(0);
		nal.write(1);
		nal.write(NAL_TYPE_SEI);
		writeEscaped(nal, rbsp.toByteArray());
		return nal.toByteArray();
	}

	/**
	 * Absolute index of the start code in front of the first slice NAL unit
	 * between frame's position and limit, or -1 when it holds none.
	 */
	static int firstSliceAt(ByteBuffer frame) {
		int end = frame.limit();
		for (int i = frame.position(); i + 3 < end; i++) {
			if (frame.get(i) != 0 || frame.get(i + 1) != 0 || frame.get(i + 2) != 1) {
				continue;
			}
			int type = frame.get(i + 3) & 0x1F;
			if (type == NAL_TYPE_SLICE || type == NAL_TYPE_IDR) {
				// a 4-byte start code begins one zero earlier
				boolean longStartCode = i > frame.position() && frame.get(i - 1) == 0;
				return longStartCode ? i - 1 : i;
			}
		}
		return -1;
	}

	/** Writes rbsp with emulation prevention bytes, so no start code appears inside it. */
	private static void writeEscaped(ByteArrayOutputStream out, byte[] rbsp) {
		int zeros = 0;
		for (byte b : rbsp) {
			int v = b & 0xFF;
			if (zeros >= 2 && v <= 3) {
				out.write(3);
				zeros = 0;
			}
			out.write(v);
			zeros = v == 0 ? zeros + 1 : 0;
		}
	}
}
