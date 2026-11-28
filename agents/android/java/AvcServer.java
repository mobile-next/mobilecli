package com.mobilenext.mobilecli;

import android.media.MediaCodec;
import android.media.MediaCodecInfo;
import android.media.MediaFormat;
import android.os.Bundle;
import android.os.SystemClock;
import android.util.Log;

import org.json.JSONObject;

import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.CountDownLatch;

/**
 * Streams the screen as a raw H.264 elementary stream to stdout via app_process,
 * with live encoder control (bitrate, keyframe) over a localabstract JSON-RPC
 * socket. Stdin can't carry control: the host launches this with `adb exec-out`,
 * which does not forward stdin.
 *
 * Usage:
 *   adb exec-out CLASSPATH=/data/local/tmp/mobilecli.dex app_process / \
 *     com.mobilenext.mobilecli.AvcServer [--bitrate bps] [--scale 0.1-2.0] [--fps 1-60]
 *
 * Control methods: screencapture.setBitrate {bps}, screencapture.requestKeyFrame,
 * and screencapture.stop, which replies and then shuts the server down cleanly
 * (exit 0). Killing the host's adb alone leaves this process running until its
 * next write fails, which a static screen may never make.
 *
 * Only one AvcServer runs per device: the control socket name is fixed, and a
 * second server would take keyframe and bitrate requests meant for the first.
 * Startup fails when the socket is already held.
 *
 * Every picture is preceded by a timecode SEI (see AvcTimecode). When the
 * display is larger than the encoder supports, it is encoded at the largest
 * size the encoder takes, keeping the aspect ratio (see EncoderSize).
 */
public class AvcServer {

	private static final String TAG = "AvcServer";
	private static final int DEFAULT_BITRATE = 3_000_000; // 3 Mbps — sane ceiling for mirroring over constrained links
	private static final int MIN_BITRATE = 100_000;
	private static final int MAX_BITRATE = 10_000_000;
	private static final int I_FRAME_INTERVAL = 2; // seconds
	private static final long DEQUEUE_TIMEOUT_US = 100_000; // matches REPEAT_PREVIOUS_FRAME_AFTER
	// The encoder is "idle" when it produced nothing for this long: the screen is static and
	// REPEAT_PREVIOUS_FRAME_AFTER has stopped feeding it (it repeats a frame only a few times).
	private static final long IDLE_AFTER_MS = 500;
	// A new virtual display is drawn once when it appears, but not when the screen is
	// dozing or the first draw was missed; until the first output, it is redrawn this
	// often, this many times, so a fresh stream does not wait for the screen to change.
	private static final long FIRST_OUTPUT_REFRESH_MS = 1_000;
	private static final int FIRST_OUTPUT_REFRESHES = 3;

	// localabstract socket for the control channel; must match avcControlSocket in devices/avc_control.go
	static final String CONTROL_SOCKET = "mobilecli-avc";

	private final int bitrate;
	private final float scale;
	private final int fps;
	private final CountDownLatch shutdown = new CountDownLatch(1);
	private JsonRpcSocketServer controlServer;

	// MediaCodec is driven from the encoder thread; control-socket commands are
	// enqueued here and drained there so all codec access stays single-threaded.
	private final ConcurrentLinkedQueue<Runnable> codecCommands = new ConcurrentLinkedQueue<>();
	private volatile long lastOutputAtMs = SystemClock.uptimeMillis();
	// reused for every stamped picture, so each one leaves in a single write
	private ByteBuffer stamped = ByteBuffer.allocateDirect(0);

	AvcServer(int bitrate, float scale, int fps) {
		this.bitrate = bitrate;
		this.scale = scale;
		this.fps = fps;
	}

	public static void main(String[] args) {
		int bitrate = DEFAULT_BITRATE;
		float scale = 1.0f;
		int fps = 30;
		for (int i = 0; i + 1 < args.length; i += 2) {
			if (args[i].equals("--bitrate")) {
				bitrate = clampBitrate(Integer.parseInt(args[i + 1]));
			} else if (args[i].equals("--scale")) {
				scale = Math.max(0.1f, Math.min(2.0f, Float.parseFloat(args[i + 1])));
			} else if (args[i].equals("--fps")) {
				fps = Math.max(1, Math.min(60, Integer.parseInt(args[i + 1])));
			}
		}
		try {
			new AvcServer(bitrate, scale, fps).stream();
		} catch (Exception e) {
			Log.e(TAG, "Failed to start AVC stream", e);
			System.err.println("Error: " + e.getMessage());
			System.exit(1);
		}
		// binder and other runtime threads must not keep the process alive
		System.exit(0);
	}

	private static int clampBitrate(int bps) {
		return Math.max(MIN_BITRATE, Math.min(MAX_BITRATE, bps));
	}

	private void startControlServer(MediaCodec codec, DisplayUtils.DisplayMirror mirror) throws IOException {
		controlServer = new JsonRpcSocketServer(CONTROL_SOCKET, (method, params) -> {
			switch (method) {
				case "screencapture.setBitrate": {
					int bps = params == null ? -1 : params.optInt("bps", -1);
					if (bps <= 0) {
						throw new RpcException(RpcException.INTERNAL_ERROR, "setBitrate requires a positive 'bps'");
					}
					int clamped = clampBitrate(bps);
					codecCommands.add(() -> {
						Bundle b = new Bundle();
						b.putInt(MediaCodec.PARAMETER_KEY_VIDEO_BITRATE, clamped);
						codec.setParameters(b);
					});
					return new JSONObject().put("bitrate", clamped);
				}
				case "screencapture.requestKeyFrame": {
					codecCommands.add(() -> {
						Bundle b = new Bundle();
						b.putInt(MediaCodec.PARAMETER_KEY_REQUEST_SYNC_FRAME, 0);
						codec.setParameters(b);
						// A sync frame is made from the next input frame, and a static screen
						// sends none: have the display drawn once more so there is one.
						if (SystemClock.uptimeMillis() - lastOutputAtMs > IDLE_AFTER_MS) {
							mirror.refresh();
						}
					});
					return new JSONObject().put("ok", true);
				}
				case "screencapture.stop": {
					// the encode loop polls the latch every DEQUEUE_TIMEOUT_US, so this ends
					// it on a static screen too; counting down again is a no-op
					controlServer.afterReply(shutdown::countDown);
					return new JSONObject().put("ok", true);
				}
				default:
					throw new RpcException(RpcException.METHOD_NOT_FOUND, "Method not found: " + method);
			}
		});
		controlServer.bindAndServeDaemon();
	}

	/**
	 * Writes one encoder output buffer in a single write. A buffer that holds a
	 * picture (any slice) gets a timecode SEI in front of its first slice; one
	 * without a slice, such as the SPS/PPS config buffer, is written as is.
	 */
	private void writeOutput(FileChannel out, ByteBuffer frame, long timestampUs) throws IOException {
		int sliceAt = AvcTimecode.firstSliceAt(frame);
		if (sliceAt < 0) {
			writeFully(out, frame);
			return;
		}

		byte[] sei = AvcTimecode.sei(timestampUs);
		int size = frame.remaining() + sei.length;
		if (stamped.capacity() < size) {
			stamped = ByteBuffer.allocateDirect(size + size / 2);
		}
		stamped.clear();
		ByteBuffer head = frame.duplicate();
		head.limit(sliceAt);
		stamped.put(head);
		stamped.put(sei);
		frame.position(sliceAt);
		stamped.put(frame);
		stamped.flip();
		writeFully(out, stamped);
	}

	// blocking zero-copy write; provides backpressure
	private static void writeFully(FileChannel out, ByteBuffer buffer) throws IOException {
		while (buffer.hasRemaining()) {
			out.write(buffer);
		}
	}

	private void stream() throws Exception {
		Runtime.getRuntime().addShutdownHook(new Thread(shutdown::countDown));

		DisplayUtils.DisplayInfo display = DisplayUtils.getDisplayInfo();
		MediaCodec codec = MediaCodec.createEncoderByType(MediaFormat.MIMETYPE_VIDEO_AVC);
		MediaCodecInfo.VideoCapabilities caps = codec.getCodecInfo()
				.getCapabilitiesForType(MediaFormat.MIMETYPE_VIDEO_AVC).getVideoCapabilities();

		// H.264 encoders only accept dimensions that are a multiple of their
		// alignment (usually 2); a raw (display * scale) can be odd. Round down.
		int widthAlign = Math.max(caps.getWidthAlignment(), 2);
		int heightAlign = Math.max(caps.getHeightAlignment(), 2);
		int requestedWidth = (int) (display.width * scale);
		int requestedHeight = (int) (display.height * scale);
		int[] size = EncoderSize.fit(requestedWidth, requestedHeight, widthAlign, heightAlign, caps::isSizeSupported);
		if (size == null) {
			codec.release();
			throw new IllegalArgumentException("No video size the encoder supports fits "
					+ requestedWidth + "x" + requestedHeight);
		}
		int width = size[0];
		int height = size[1];
		if (width != requestedWidth || height != requestedHeight) {
			// logcat only: stdout and stderr carry the stream
			Log.i(TAG, "Encoding " + requestedWidth + "x" + requestedHeight + " as " + width + "x" + height);
		}

		MediaFormat format = MediaFormat.createVideoFormat(MediaFormat.MIMETYPE_VIDEO_AVC, width, height);
		format.setInteger(MediaFormat.KEY_BIT_RATE, bitrate);
		// CBR caps the encoder's peak rate so a burst of motion can't flood a
		// constrained viewer link; a static screen still idles low.
		format.setInteger(MediaFormat.KEY_BITRATE_MODE, MediaCodecInfo.EncoderCapabilities.BITRATE_MODE_CBR);
		format.setInteger(MediaFormat.KEY_FRAME_RATE, fps);
		format.setInteger(MediaFormat.KEY_CAPTURE_RATE, fps);
		format.setFloat(MediaFormat.KEY_OPERATING_RATE, fps);
		format.setInteger(MediaFormat.KEY_I_FRAME_INTERVAL, I_FRAME_INTERVAL);
		format.setInteger(MediaFormat.KEY_COLOR_FORMAT, MediaCodecInfo.CodecCapabilities.COLOR_FormatSurface);
		format.setInteger(MediaFormat.KEY_PROFILE, MediaCodecInfo.CodecProfileLevel.AVCProfileHigh);
		format.setInteger(MediaFormat.KEY_LATENCY, 0);
		format.setInteger(MediaFormat.KEY_PRIORITY, 0); // realtime
		// keep the stream alive when the screen is static
		format.setLong(MediaFormat.KEY_REPEAT_PREVIOUS_FRAME_AFTER, 100_000L);

		try {
			codec.configure(format, null, null, MediaCodec.CONFIGURE_FLAG_ENCODE);
		} catch (Exception e) {
			codec.release();
			throw e;
		}

		DisplayUtils.DisplayMirror virtualDisplay = DisplayUtils.createVirtualDisplay(
				"avc.screen.capture", width, height, codec.createInputSurface());
		if (virtualDisplay == null) {
			codec.release();
			throw new IllegalStateException("Failed to create virtual display");
		}

		try {
			startControlServer(codec, virtualDisplay);
		} catch (IOException e) {
			virtualDisplay.release();
			codec.release();
			throw new IllegalStateException("another screen capture is already running on this device ("
					+ CONTROL_SOCKET + " is taken)", e);
		}
		codec.start();

		MediaCodec.BufferInfo info = new MediaCodec.BufferInfo();
		FileChannel stdout = new FileOutputStream(FileDescriptor.out).getChannel();
		boolean hadOutput = false;
		int firstOutputRefreshes = 0;
		long lastRefreshAtMs = SystemClock.uptimeMillis();
		try {
			while (shutdown.getCount() > 0) {
				Runnable command;
				while ((command = codecCommands.poll()) != null) {
					try {
						command.run();
					} catch (Exception e) {
						Log.e(TAG, "Error applying codec command", e);
					}
				}

				int index = codec.dequeueOutputBuffer(info, DEQUEUE_TIMEOUT_US);
				if (index < 0) {
					// INFO_TRY_AGAIN_LATER / INFO_OUTPUT_FORMAT_CHANGED
					long now = SystemClock.uptimeMillis();
					if (!hadOutput && firstOutputRefreshes < FIRST_OUTPUT_REFRESHES
							&& now - lastRefreshAtMs >= FIRST_OUTPUT_REFRESH_MS) {
						virtualDisplay.refresh();
						firstOutputRefreshes++;
						lastRefreshAtMs = now;
					}
					continue;
				}
				// the config buffer (SPS/PPS) can come before any picture
				if ((info.flags & MediaCodec.BUFFER_FLAG_CODEC_CONFIG) == 0) {
					hadOutput = true;
				}
				ByteBuffer buffer = codec.getOutputBuffer(index);
				if (buffer != null && info.size > 0) {
					buffer.position(info.offset);
					buffer.limit(info.offset + info.size);
					try {
						writeOutput(stdout, buffer, info.presentationTimeUs);
					} catch (IOException e) {
						break; // pipe broken - client disconnected
					}
				}
				codec.releaseOutputBuffer(index, false);
				lastOutputAtMs = SystemClock.uptimeMillis();
			}
		} finally {
			try { stdout.close(); } catch (Exception ignored) { }
			try { codec.stop(); } catch (Exception ignored) { }
			try { codec.release(); } catch (Exception ignored) { }
			try { virtualDisplay.release(); } catch (Exception ignored) { }
			controlServer.stop();
		}
	}
}
