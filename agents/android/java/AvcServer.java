package com.mobilenext.mobilecli;

import android.hardware.display.VirtualDisplay;
import android.media.MediaCodec;
import android.media.MediaCodecInfo;
import android.media.MediaFormat;
import android.os.Bundle;
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
 */
public class AvcServer {

	private static final String TAG = "AvcServer";
	private static final int DEFAULT_BITRATE = 3_000_000; // 3 Mbps — sane ceiling for mirroring over constrained links
	private static final int MIN_BITRATE = 100_000;
	private static final int MAX_BITRATE = 10_000_000;
	private static final int I_FRAME_INTERVAL = 2; // seconds
	private static final long DEQUEUE_TIMEOUT_US = 100_000; // matches REPEAT_PREVIOUS_FRAME_AFTER

	// localabstract socket for the control channel; must match avcControlSocket in devices/avc_control.go
	static final String CONTROL_SOCKET = "mobilecli-avc";

	private final int bitrate;
	private final float scale;
	private final int fps;
	private final CountDownLatch shutdown = new CountDownLatch(1);

	// MediaCodec is driven from the encoder thread; control-socket commands are
	// enqueued here and drained there so all codec access stays single-threaded.
	private final ConcurrentLinkedQueue<Runnable> codecCommands = new ConcurrentLinkedQueue<>();

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
	}

	private static int clampBitrate(int bps) {
		return Math.max(MIN_BITRATE, Math.min(MAX_BITRATE, bps));
	}

	private void startControlServer(MediaCodec codec) {
		new JsonRpcSocketServer(CONTROL_SOCKET, (method, params) -> {
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
					});
					return new JSONObject().put("ok", true);
				}
				default:
					throw new RpcException(RpcException.METHOD_NOT_FOUND, "Method not found: " + method);
			}
		}).startDaemon();
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
		int width = ((int) (display.width * scale) / widthAlign) * widthAlign;
		int height = ((int) (display.height * scale) / heightAlign) * heightAlign;

		if (width <= 0 || height <= 0) {
			codec.release();
			throw new IllegalArgumentException("Invalid dimensions: " + width + "x" + height);
		}
		if (!caps.isSizeSupported(width, height)) {
			String max = caps.getSupportedWidths().getUpper() + "x" + caps.getSupportedHeights().getUpper();
			codec.release();
			throw new IllegalArgumentException("Video dimensions " + width + "x" + height
					+ " exceed codec capabilities. Maximum supported: " + max
					+ ". Try using --scale parameter to reduce resolution (e.g., --scale 0.5)");
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

		VirtualDisplay virtualDisplay = DisplayUtils.createVirtualDisplay(
				"avc.screen.capture", width, height, display.dpi, codec.createInputSurface());
		if (virtualDisplay == null) {
			codec.release();
			throw new IllegalStateException("Failed to create virtual display");
		}

		codec.start();
		startControlServer(codec);

		MediaCodec.BufferInfo info = new MediaCodec.BufferInfo();
		FileChannel stdout = new FileOutputStream(FileDescriptor.out).getChannel();
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
					continue; // INFO_TRY_AGAIN_LATER / INFO_OUTPUT_FORMAT_CHANGED
				}
				ByteBuffer buffer = codec.getOutputBuffer(index);
				if (buffer != null && info.size > 0) {
					buffer.position(info.offset);
					buffer.limit(info.offset + info.size);
					try {
						// blocking zero-copy write; provides backpressure
						while (buffer.hasRemaining()) {
							stdout.write(buffer);
						}
					} catch (IOException e) {
						break; // pipe broken - client disconnected
					}
				}
				codec.releaseOutputBuffer(index, false);
			}
		} finally {
			try { stdout.close(); } catch (Exception ignored) { }
			try { codec.stop(); } catch (Exception ignored) { }
			try { codec.release(); } catch (Exception ignored) { }
			try { virtualDisplay.release(); } catch (Exception ignored) { }
		}
	}
}
