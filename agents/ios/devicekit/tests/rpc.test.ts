import {
  test,
  expect,
  request as playwrightRequest,
  type APIRequestContext,
} from "@playwright/test";
import { EXIF_UPRIGHT, exifOrientationOf, sizeOf, type ImageSize } from "./image";

const BASE_URL = "http://localhost:12004";

// rotates with the device, which the home screen of an iphone does not
const SAFARI_BUNDLE_ID = "com.apple.mobilesafari";

type RpcError = { code: number; message?: string };
type RpcResponse = {
  jsonrpc: string;
  id: number;
  result?: any;
  error?: RpcError;
};

let requestId = 0;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function rpc(
  request: APIRequestContext,
  method: string,
  params: Record<string, any> = {},
): Promise<RpcResponse> {
  const res = await request.post(`${BASE_URL}/rpc`, {
    headers: { "Content-Type": "application/json" },
    data: {
      jsonrpc: "2.0",
      method,
      params,
      id: ++requestId,
    },
  });
  return (await res.json()) as RpcResponse;
}

function returnsResult(response: RpcResponse): any {
  expect(response.jsonrpc).toBe("2.0");
  expect(
    response.result,
    "expected result, got error: " + JSON.stringify(response.error),
  ).toBeDefined();
  return response.result;
}

function returnsError(response: RpcResponse): RpcError {
  expect(response.jsonrpc).toBe("2.0");
  expect(response.error, "expected error, got result").toBeDefined();
  return response.error!;
}

async function takeScreenshot(request: APIRequestContext, format: string): Promise<Buffer> {
  const result = returnsResult(await rpc(request, "device.screenshot", { format }));
  const base64 = (result.data as string).split(",")[1];
  return Buffer.from(base64, "base64");
}

// the size of the app on screen in points: the space taps and the ui tree use
async function foregroundAppSize(request: APIRequestContext): Promise<ImageSize> {
  const tree = returnsResult(await rpc(request, "device.dump.ui"));
  const app = (tree.children as any[]).find((child) => child.type === "XCUIElementTypeApplication");
  expect(app, "no application in the ui tree").toBeDefined();
  return { width: app.rect.width, height: app.rect.height };
}

async function foregroundAppSizeInPixels(request: APIRequestContext): Promise<ImageSize> {
  const { scale } = returnsResult(await rpc(request, "device.info"));
  const { width, height } = await foregroundAppSize(request);
  return { width: width * scale, height: height * scale };
}

async function isForegroundAppInLandscape(request: APIRequestContext): Promise<boolean> {
  const { width, height } = await foregroundAppSize(request);
  return width > height;
}

async function showSafariInLandscape(request: APIRequestContext): Promise<void> {
  returnsResult(await rpc(request, "device.apps.launch", { bundleId: SAFARI_BUNDLE_ID }));
  returnsResult(await rpc(request, "device.io.orientation.set", { orientation: "LANDSCAPE" }));

  // the app rotates a moment after the device does
  await expect
    .poll(() => isForegroundAppInLandscape(request), { message: "safari never rotated to landscape" })
    .toBe(true);
}

async function returnToPortraitHomeScreen(request: APIRequestContext): Promise<void> {
  returnsResult(await rpc(request, "device.io.orientation.set", { orientation: "PORTRAIT" }));
  returnsResult(await rpc(request, "device.io.button", { button: "home" }));
}

// ---------------------------------------------------------------------------
// device.info
// ---------------------------------------------------------------------------
test.describe("device.info", () => {
  test("returns screen size and scale", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.info"));
    expect(result.screenSize).toBeTruthy();
    expect(typeof result.screenSize.width).toBe("number");
    expect(typeof result.screenSize.height).toBe("number");
    expect(result.screenSize.width).toBeGreaterThan(0);
    expect(result.screenSize.height).toBeGreaterThan(0);
    expect(typeof result.scale).toBe("number");
    expect(result.scale).toBeGreaterThan(0);
  });

  test("ignores extra params", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.info", { foo: "bar" }));
    expect(result.screenSize).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.apps.foreground
// ---------------------------------------------------------------------------
test.describe("device.apps.foreground", () => {
  test("returns foreground app info", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.apps.foreground"));
    expect(result).toHaveProperty("bundleId");
    expect(result).toHaveProperty("name");
    expect(result).toHaveProperty("pid");
  });

  test("reports the view controller on screen", async ({ request }) => {
    await rpc(request, "device.apps.launch", { bundleId: "com.apple.Preferences" });
    await sleep(2000);

    try {
      const result = returnsResult(await rpc(request, "device.apps.foreground"));
      expect(result.bundleId).toBe("com.apple.Preferences");
      expect(typeof result.viewController).toBe("string");
      expect(result.viewController.length).toBeGreaterThan(0);
    } finally {
      // leaving Settings in the foreground would change what later tests see
      await rpc(request, "device.apps.terminate", { bundleId: "com.apple.Preferences" });
    }
  });
});

// ---------------------------------------------------------------------------
// device.apps.launch & device.apps.terminate
// ---------------------------------------------------------------------------
test.describe("device.apps.launch and device.apps.terminate", () => {
  test("fails to launch without bundleId", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.apps.launch"));
    expect(error.code).toBeTruthy();
  });

  test("fails to terminate without bundleId", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.apps.terminate"));
    expect(error.code).toBeTruthy();
  });

  test("terminating a non-running app returns terminated false", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.apps.terminate", { bundleId: "com.invalid.nonexistent" }),
    );
    expect(result.terminated).toBe(false);
  });

  test("launch settings, verify foreground, terminate, verify springboard, terminate again returns false", async ({ request }) => {
    returnsResult(await rpc(request, "device.apps.launch", { bundleId: "com.apple.Preferences" }));
    await sleep(1000);

    const afterLaunch = returnsResult(await rpc(request, "device.apps.foreground"));
    expect(afterLaunch.bundleId).toBe("com.apple.Preferences");

    const first = returnsResult(await rpc(request, "device.apps.terminate", { bundleId: "com.apple.Preferences" }));
    expect(first.terminated).toBe(true);
    await sleep(1000);

    const afterTerminate = returnsResult(await rpc(request, "device.apps.foreground"));
    expect(afterTerminate.bundleId).toBe("com.apple.springboard");

    const second = returnsResult(await rpc(request, "device.apps.terminate", { bundleId: "com.apple.Preferences" }));
    expect(second.terminated).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// device.io.tap
// ---------------------------------------------------------------------------
test.describe("device.io.tap", () => {
  test("taps at coordinates", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.io.tap", { x: 100, y: 100 }));
    expect(result).toBeTruthy();
  });

  test("fails without coordinates", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.tap"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.io.swipe
// ---------------------------------------------------------------------------
test.describe("device.io.swipe", () => {
  test("swipes between two points", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.io.swipe", { x1: 200, y1: 400, x2: 200, y2: 200 }),
    );
    expect(result).toBeTruthy();
  });

  test("swipes over an explicit duration", async ({ request }) => {
    const started = Date.now();
    const result = returnsResult(
      await rpc(request, "device.io.swipe", {
        x1: 200,
        y1: 400,
        x2: 200,
        y2: 200,
        duration: 1.5,
      }),
    );
    expect(result).toBeTruthy();
    expect(Date.now() - started).toBeGreaterThanOrEqual(1400);
  });

  test("fails without coordinates", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.swipe"));
    expect(error.code).toBeTruthy();
  });

  test("rejects a negative duration", async ({ request }) => {
    const error = returnsError(
      await rpc(request, "device.io.swipe", {
        x1: 200,
        y1: 400,
        x2: 200,
        y2: 200,
        duration: -1,
      }),
    );
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.io.longpress
// ---------------------------------------------------------------------------
test.describe("device.io.longpress", () => {
  test("long presses at coordinates", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.io.longpress", { x: 100, y: 100, duration: 0.5 }),
    );
    expect(result).toBeTruthy();
  });

  test("fails without params", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.longpress"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.io.gesture
// ---------------------------------------------------------------------------
test.describe("device.io.gesture", () => {
  test("performs a tap gesture via actions", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.io.gesture", {
        actions: [
          { type: "press", x: 150, y: 150, duration: 0, button: 0 },
          { type: "release", x: 150, y: 150, duration: 0.1, button: 0 },
        ],
      }),
    );
    expect(result).toBeTruthy();
  });

  test("fails without actions", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.gesture"));
    expect(error.code).toBeTruthy();
  });

  test("taps twice with a second finger that starts after the first lifts", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.io.gesture", {
        actions: [
          { type: "press", x: 150, y: 150, duration: 0.05, button: 0 },
          { type: "release", x: 150, y: 150, duration: 0, button: 0 },
          { type: "press", x: 150, y: 150, duration: 0.05, button: 1, start: 0.2 },
          { type: "release", x: 150, y: 150, duration: 0, button: 1 },
        ],
      }),
    );
    expect(result).toBeTruthy();
  });

  test("rejects a negative start", async ({ request }) => {
    const error = returnsError(
      await rpc(request, "device.io.gesture", {
        actions: [
          { type: "press", x: 150, y: 150, duration: 0, button: 0, start: -1 },
          { type: "release", x: 150, y: 150, duration: 0, button: 0 },
        ],
      }),
    );
    expect(error.code).toBe(-32602);
  });

  test("rejects start on anything but the press", async ({ request }) => {
    const error = returnsError(
      await rpc(request, "device.io.gesture", {
        actions: [
          { type: "press", x: 150, y: 150, duration: 0, button: 0 },
          { type: "release", x: 150, y: 150, duration: 0, button: 0, start: 0.5 },
        ],
      }),
    );
    expect(error.code).toBe(-32602);
  });
});

// ---------------------------------------------------------------------------
// device.io.text
// ---------------------------------------------------------------------------
test.describe("device.io.text", () => {
  test("types text", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.io.text", { text: "hello" }));
    expect(result).toBeTruthy();
  });

  test("fails without text", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.text"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.io.button
// ---------------------------------------------------------------------------
test.describe("device.io.button", () => {
  test("pressing home returns to springboard", async ({ request }) => {
    await rpc(request, "device.apps.launch", { bundleId: "com.apple.Preferences" });
    await sleep(1000);
    const before = returnsResult(await rpc(request, "device.apps.foreground"));
    expect(before.bundleId).toBe("com.apple.Preferences");

    returnsResult(await rpc(request, "device.io.button", { button: "home" }));
    await sleep(1000);

    const after = returnsResult(await rpc(request, "device.apps.foreground"));
    expect(after.bundleId).toBe("com.apple.springboard");
  });

  test("fails without button param", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.button"));
    expect(error.code).toBeTruthy();
  });

  test("fails with uppercase HOME", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.button", { button: "HOME" }));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.io.orientation
// ---------------------------------------------------------------------------
test.describe("device.io.orientation", () => {
  test.afterEach(async ({ request }) => {
    await returnToPortraitHomeScreen(request);
  });

  // get reports the orientation the foreground app is drawn in, and the app
  // rotates a moment after the device does
  test("sets orientation to landscape and back", async ({ request }) => {
    const orientation = async () => returnsResult(await rpc(request, "device.io.orientation.get")).orientation;

    await showSafariInLandscape(request);
    await expect.poll(orientation).toBe("LANDSCAPE");

    returnsResult(await rpc(request, "device.io.orientation.set", { orientation: "PORTRAIT" }));
    await expect.poll(orientation).toBe("PORTRAIT");
  });

  test("fails without orientation param", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.io.orientation.set"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.screenshot
// ---------------------------------------------------------------------------
test.describe("device.screenshot", () => {
  test("captures a png screenshot", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.screenshot", { format: "png" }));
    expect(typeof result.data).toBe("string");
    expect(result.data.length).toBeGreaterThan(0);
  });

  test("captures a jpeg screenshot", async ({ request }) => {
    const result = returnsResult(
      await rpc(request, "device.screenshot", { format: "jpeg", quality: 50 }),
    );
    expect(typeof result.data).toBe("string");
    expect(result.data.length).toBeGreaterThan(0);
  });

  test("fails without format", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.screenshot"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.screenshot in landscape
// ---------------------------------------------------------------------------
test.describe("device.screenshot in landscape", () => {
  test.beforeEach(async ({ request }) => {
    await showSafariInLandscape(request);
  });

  test.afterEach(async ({ request }) => {
    await returnToPortraitHomeScreen(request);
  });

  for (const format of ["png", "jpeg"]) {
    // taps and the ui tree are expressed in the rotated screen, so a screenshot
    // that is not has every coordinate read off it land somewhere else
    test(`a ${format} screenshot is as wide and as tall as the rotated screen`, async ({ request }) => {
      const screenshot = await takeScreenshot(request, format);
      expect(sizeOf(screenshot)).toEqual(await foregroundAppSizeInPixels(request));
    });

    // the rotation must be in the pixels: a tag asking the viewer to rotate is
    // ignored by some decoders and applied by others
    test(`a ${format} screenshot does not ask its viewer to rotate it`, async ({ request }) => {
      const screenshot = await takeScreenshot(request, format);
      expect(exifOrientationOf(screenshot)).toBe(EXIF_UPRIGHT);
    });
  }

  // clients scale screenshot positions by screenSize to get tap coordinates
  test("device.info reports the rotated screen size", async ({ request }) => {
    const { screenSize } = returnsResult(await rpc(request, "device.info"));
    expect(screenSize).toEqual(await foregroundAppSize(request));
  });

  test("a screenshot is device.info's screen size times its scale", async ({ request }) => {
    const { screenSize, scale } = returnsResult(await rpc(request, "device.info"));
    const screenshot = await takeScreenshot(request, "png");
    expect(sizeOf(screenshot)).toEqual({ width: screenSize.width * scale, height: screenSize.height * scale });
  });
});

// ---------------------------------------------------------------------------
// device.dump.ui
// ---------------------------------------------------------------------------
test.describe("device.dump.ui", () => {
  test("dumps the UI hierarchy without params", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.dump.ui"));
    expect(result).toBeTruthy();
  });

  test("returns source tree format for json", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.dump.ui", { format: "json" }));
    expect(typeof result.type, "expected type to be a string").toBe("string");
    expect(result.rect, "expected rect").toBeTruthy();
    expect(typeof result.rect.x, "expected rect.x").toBe("number");
    expect(typeof result.rect.y, "expected rect.y").toBe("number");
    expect(typeof result.rect.width, "expected rect.width").toBe("number");
    expect(typeof result.rect.height, "expected rect.height").toBe("number");
    expect("elementType" in result, "should not have elementType").toBe(false);
    expect("frame" in result, "should not have frame").toBe(false);
    expect("isVisible" in result, "should not have isVisible").toBe(false);
  });

  test("json children follow the same source tree format", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.dump.ui", { format: "json" }));
    expect(Array.isArray(result.children), "expected children array").toBe(true);
    expect(result.children.length, "expected at least one child").toBeGreaterThan(0);
    const child = result.children[0];
    expect(typeof child.type, "child should have string type").toBe("string");
    expect(child.rect, "child should have rect").toBeTruthy();
    expect(typeof child.rect.x, "child rect.x should be number").toBe("number");
  });

  test("dumps the UI hierarchy as raw", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.dump.ui", { format: "raw" }));
    expect(typeof result.elementType, "raw format should have numeric elementType").toBe("number");
    expect(result.frame, "raw format should have frame").toBeTruthy();
  });
});

function elementsOfType(element: any, type: string): any[] {
  const own = element.type === type ? [element] : [];
  const children = (element.children ?? []) as any[];
  return own.concat(...children.map((child) => elementsOfType(child, type)));
}

// springboard reports its own elements on the panel's portrait axes, whatever
// way the home screen is drawn
test.describe("device.dump.ui on the home screen in landscape", () => {
  test.beforeEach(async ({ request }) => {
    returnsResult(await rpc(request, "device.io.button", { button: "home" }));
    returnsResult(await rpc(request, "device.io.orientation.set", { orientation: "LANDSCAPE" }));
    // only an ipad's home screen turns, so there is no rotation to wait for
    await sleep(2000);
  });

  test.afterEach(async ({ request }) => {
    await returnToPortraitHomeScreen(request);
  });

  test("tapping the middle of an icon opens its app", async ({ request }) => {
    const tree = returnsResult(await rpc(request, "device.dump.ui"));
    const safari = elementsOfType(tree, "XCUIElementTypeIcon")
      .find((icon) => icon.label === "Safari" && icon.rect.width > 0);
    expect(safari, "no Safari icon on the home screen").toBeDefined();

    const { x, y, width, height } = safari.rect;
    returnsResult(await rpc(request, "device.io.tap", { x: x + width / 2, y: y + height / 2 }));

    await expect
      .poll(async () => returnsResult(await rpc(request, "device.apps.foreground")).bundleId)
      .toBe(SAFARI_BUNDLE_ID);
  });

  test("the status bar runs along the top of the screen", async ({ request }) => {
    const { screenSize } = returnsResult(await rpc(request, "device.info"));
    const tree = returnsResult(await rpc(request, "device.dump.ui"));
    const statusBars = elementsOfType(tree, "XCUIElementTypeStatusBar").filter((bar) => bar.rect.width > 0);
    expect(statusBars.length, "no status bar in the ui tree").toBeGreaterThan(0);

    for (const { rect } of statusBars) {
      expect({ x: rect.x, y: rect.y, width: rect.width }).toEqual({ x: 0, y: 0, width: screenSize.width });
    }
  });

  // the application element itself already reports the rotated screen
  test("the home screen fills the screen", async ({ request }) => {
    const { screenSize } = returnsResult(await rpc(request, "device.info"));
    const tree = returnsResult(await rpc(request, "device.dump.ui"));
    const [springboard] = elementsOfType(tree, "XCUIElementTypeApplication");
    expect(springboard, "no application in the ui tree").toBeDefined();
    expect(springboard.rect).toEqual({ x: 0, y: 0, ...screenSize });
  });
});

// ---------------------------------------------------------------------------
// device.url
// ---------------------------------------------------------------------------
test.describe("device.url", () => {
  test("opens an https url", async ({ request }) => {
    const result = returnsResult(await rpc(request, "device.url", { url: "https://www.apple.com" }));
    expect(result).toBeTruthy();
  });

  test("fails without url param", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.url"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.clipboard
// ---------------------------------------------------------------------------
test.describe("device.clipboard", () => {
  test("reads back what it wrote", async ({ request }) => {
    const text = `devicekit-${Date.now()}`;
    returnsResult(await rpc(request, "device.clipboard.set", { text }));
    const result = returnsResult(await rpc(request, "device.clipboard.get"));
    expect(result.text).toBe(text);
  });

  test("returns empty text after setting an empty clipboard", async ({ request }) => {
    returnsResult(await rpc(request, "device.clipboard.set", { text: "" }));
    const result = returnsResult(await rpc(request, "device.clipboard.get"));
    expect(result.text).toBe("");
  });

  test("fails without text", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.clipboard.set"));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// device.settings.apply
// ---------------------------------------------------------------------------
test.describe("device.settings.apply", () => {
  test("switches appearance to dark and back to light", async ({ request }) => {
    returnsResult(await rpc(request, "device.settings.apply", { appearance: "dark" }));
    returnsResult(await rpc(request, "device.settings.apply", { appearance: "light" }));
  });

  test("fails on an unknown appearance", async ({ request }) => {
    const error = returnsError(await rpc(request, "device.settings.apply", { appearance: "sepia" }));
    expect(error.code).toBeTruthy();
  });
});

// ---------------------------------------------------------------------------
// error handling
// ---------------------------------------------------------------------------
test.describe("error handling", () => {
  test("returns method not found for unknown method", async ({ request }) => {
    const error = returnsError(await rpc(request, "nonexistent.method"));
    expect(error.code).toBe(-32601);
  });
});

// ---------------------------------------------------------------------------
// health check
// ---------------------------------------------------------------------------
test.describe("GET /health", () => {
  test("returns OK", async ({ request }) => {
    const res = await request.get(`${BASE_URL}/health`);
    expect(res.status()).toBe(200);
    const body = await res.text();
    expect(body).toBe("OK");
  });
});

// ---------------------------------------------------------------------------
// teardown
// ---------------------------------------------------------------------------
test.afterAll(async () => {
  const context = await playwrightRequest.newContext();
  await rpc(context, "device.io.button", { button: "home" });
  await context.dispose();
});
