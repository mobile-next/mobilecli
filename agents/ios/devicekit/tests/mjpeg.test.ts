import { test, expect } from "@playwright/test";
import http from "node:http";

const BASE_URL = "http://localhost:12004";
const BOUNDARY = "--mjpeg-frame-boundary";
const JPEG_SOI = Buffer.from([0xff, 0xd8]); // JPEG Start Of Image marker

type StreamResult = { res: http.IncomingMessage; body: Buffer };
type MjpegFrame = { headers: Record<string, string>; jpegData: Buffer };

/**
 * Connects to the MJPEG stream and collects data for `durationMs`,
 * then destroys the socket and returns the response + accumulated buffer.
 */
function collectStreamBytes(
  path: string,
  { durationMs = 2000 }: { durationMs?: number } = {},
): Promise<StreamResult> {
  const url = `${BASE_URL}${path}`;
  return new Promise((resolve, reject) => {
    const req = http.get(url, (res) => {
      const chunks: Buffer[] = [];
      let resolved = false;

      const finish = () => {
        if (resolved) return;
        resolved = true;
        req.destroy();
        resolve({ res, body: Buffer.concat(chunks) });
      };

      res.on("data", (chunk: Buffer) => chunks.push(chunk));
      res.on("end", finish);
      res.on("error", (err: NodeJS.ErrnoException) => {
        if (err.code === "ECONNRESET") return finish();
        if (!resolved) {
          resolved = true;
          reject(err);
        }
      });

      setTimeout(finish, durationMs);
    });

    req.on("error", (err: NodeJS.ErrnoException) => {
      if (err.code === "ECONNRESET") return;
      reject(err);
    });
  });
}

/**
 * Parses MJPEG frames from a raw buffer.
 * Each frame starts with "--mjpeg-frame-boundary\r\n" followed by headers and JPEG data.
 */
function parseMjpegFrames(buffer: Buffer): MjpegFrame[] {
  const text = buffer.toString("binary");
  const frames: MjpegFrame[] = [];
  let searchStart = 0;

  while (true) {
    const boundaryIndex = text.indexOf(BOUNDARY, searchStart);
    if (boundaryIndex === -1) break;

    const headerStart = boundaryIndex + BOUNDARY.length + 2; // skip boundary + \r\n
    const headerEnd = text.indexOf("\r\n\r\n", headerStart);
    if (headerEnd === -1) break;

    const headerBlock = text.slice(headerStart, headerEnd);
    const headers: Record<string, string> = {};
    for (const line of headerBlock.split("\r\n")) {
      const colon = line.indexOf(":");
      if (colon !== -1) {
        headers[line.slice(0, colon).trim().toLowerCase()] = line.slice(colon + 1).trim();
      }
    }

    const contentLength = parseInt(headers["content-length"], 10);
    const jpegStart = headerEnd + 4; // skip \r\n\r\n
    const jpegEnd = jpegStart + contentLength;

    if (jpegEnd > text.length) break; // incomplete frame

    const jpegData = Buffer.from(text.slice(jpegStart, jpegEnd), "binary");
    frames.push({ headers, jpegData });

    searchStart = jpegEnd + 2; // skip trailing \r\n
  }

  return frames;
}

// ---------------------------------------------------------------------------
// GET /mjpeg
// ---------------------------------------------------------------------------
test.describe("GET /mjpeg", () => {
  test.describe.configure({ timeout: 15000 });

  test("returns multipart content type with correct boundary", async () => {
    const { res } = await collectStreamBytes("/mjpeg");
    const contentType = res.headers["content-type"];
    expect(contentType).toBe("multipart/x-mixed-replace; boundary=mjpeg-frame-boundary");
  });

  test("returns no-cache headers", async () => {
    const { res } = await collectStreamBytes("/mjpeg");
    expect(res.headers["cache-control"]).toBe("no-cache, no-store, must-revalidate");
    expect(res.headers["pragma"]).toBe("no-cache");
    expect(res.headers["expires"]).toBe("0");
  });

  test("returns server header", async () => {
    const { res } = await collectStreamBytes("/mjpeg");
    expect(res.headers["server"]).toBe("DeviceKit-iOS");
  });

  test("streams valid MJPEG frames", async () => {
    const { body } = await collectStreamBytes("/mjpeg");
    const frames = parseMjpegFrames(body);

    expect(frames.length, `expected at least 1 frame, got ${frames.length}`).toBeGreaterThanOrEqual(1);

    for (const frame of frames) {
      expect(frame.headers["content-type"]).toBe("image/jpeg");
      expect(frame.headers["content-length"], "frame missing Content-Length").toBeTruthy();
      expect(frame.jpegData.length).toBe(parseInt(frame.headers["content-length"], 10));
    }
  });

  test("each frame contains valid JPEG data", async () => {
    const { body } = await collectStreamBytes("/mjpeg");
    const frames = parseMjpegFrames(body);

    expect(frames.length, "no frames received").toBeGreaterThanOrEqual(1);

    for (const frame of frames) {
      const startsWithJpegMagic = frame.jpegData[0] === JPEG_SOI[0] && frame.jpegData[1] === JPEG_SOI[1];
      expect(startsWithJpegMagic, "frame data does not start with JPEG SOI marker (0xFF 0xD8)").toBe(true);
      expect(frame.jpegData.length, "JPEG data suspiciously small").toBeGreaterThan(100);
    }
  });

  test("streams multiple frames over time", async () => {
    const { body } = await collectStreamBytes("/mjpeg");
    const frames = parseMjpegFrames(body);
    expect(frames.length, `expected at least 2 frames, got ${frames.length}`).toBeGreaterThanOrEqual(2);
  });

  test("accepts custom fps parameter", async () => {
    const { res, body } = await collectStreamBytes("/mjpeg?fps=1", { durationMs: 3000 });
    expect(res.statusCode).toBe(200);
    const frames = parseMjpegFrames(body);
    expect(frames.length, "no frames received with fps=1").toBeGreaterThanOrEqual(1);
  });

  test("accepts custom quality parameter", async () => {
    const { body: lowQ } = await collectStreamBytes("/mjpeg?quality=1");
    const { body: highQ } = await collectStreamBytes("/mjpeg?quality=100");

    const lowFrames = parseMjpegFrames(lowQ);
    const highFrames = parseMjpegFrames(highQ);

    expect(lowFrames.length, "no frames at quality=1").toBeGreaterThanOrEqual(1);
    expect(highFrames.length, "no frames at quality=100").toBeGreaterThanOrEqual(1);

    // Higher quality should produce larger JPEG data on average
    const avgLow = lowFrames.reduce((sum, f) => sum + f.jpegData.length, 0) / lowFrames.length;
    const avgHigh = highFrames.reduce((sum, f) => sum + f.jpegData.length, 0) / highFrames.length;
    expect(avgHigh, `expected quality=100 (avg ${avgHigh}B) to produce larger frames than quality=1 (avg ${avgLow}B)`).toBeGreaterThan(avgLow);
  });

  test("accepts custom scale parameter", async () => {
    const { body: fullScale } = await collectStreamBytes("/mjpeg?scale=100");
    const { body: halfScale } = await collectStreamBytes("/mjpeg?scale=50");

    const fullFrames = parseMjpegFrames(fullScale);
    const halfFrames = parseMjpegFrames(halfScale);

    expect(fullFrames.length, "no frames at scale=100").toBeGreaterThanOrEqual(1);
    expect(halfFrames.length, "no frames at scale=50").toBeGreaterThanOrEqual(1);

    // Smaller scale should produce smaller JPEG data on average
    const avgFull = fullFrames.reduce((sum, f) => sum + f.jpegData.length, 0) / fullFrames.length;
    const avgHalf = halfFrames.reduce((sum, f) => sum + f.jpegData.length, 0) / halfFrames.length;
    expect(avgFull, `expected scale=100 (avg ${avgFull}B) to produce larger frames than scale=50 (avg ${avgHalf}B)`).toBeGreaterThan(avgHalf);
  });

  test("clamps out-of-range fps values", async () => {
    // fps=0 should be clamped to 1, fps=999 should be clamped to 60 — both should still stream
    const { res: lowRes } = await collectStreamBytes("/mjpeg?fps=0");
    expect(lowRes.statusCode).toBe(200);

    const { res: highRes } = await collectStreamBytes("/mjpeg?fps=999");
    expect(highRes.statusCode).toBe(200);
  });

  test("clamps out-of-range quality values", async () => {
    const { res: lowRes } = await collectStreamBytes("/mjpeg?quality=0");
    expect(lowRes.statusCode).toBe(200);

    const { res: highRes } = await collectStreamBytes("/mjpeg?quality=999");
    expect(highRes.statusCode).toBe(200);
  });

  test("clamps out-of-range scale values", async () => {
    const { res: lowRes } = await collectStreamBytes("/mjpeg?scale=0");
    expect(lowRes.statusCode).toBe(200);

    const { res: highRes } = await collectStreamBytes("/mjpeg?scale=999");
    expect(highRes.statusCode).toBe(200);
  });
});
