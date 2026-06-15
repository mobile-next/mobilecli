package dev.mobilewright.agent;

import android.graphics.Bitmap;
import android.os.HandlerThread;
import android.os.Looper;
import android.os.SystemClock;
import android.view.InputEvent;
import android.view.KeyEvent;
import android.view.MotionEvent;
import android.view.accessibility.AccessibilityNodeInfo;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.io.OutputStream;
import java.lang.reflect.Constructor;
import java.lang.reflect.Method;

import android.net.LocalServerSocket;
import android.net.LocalSocket;

/**
 * Persistent on-device automation server.
 *
 * Runs via `app_process` as the shell uid, holds a UiAutomation connection
 * open for the lifetime of the process, and serves newline-delimited JSON
 * commands over a localabstract LocalServerSocket. Because the process and the
 * UiAutomation connection persist, there is no per-request instrumentation
 * spawn and no fixed waitForIdle, which is what makes hierarchy dumps and input
 * injection an order of magnitude faster than per-command adb.
 *
 * All framework-hidden APIs (UiAutomation, UiAutomationConnection, InputManager)
 * are reached via reflection so the agent compiles against the public SDK jar.
 */
public final class Agent {

    static final String SOCKET_NAME = "mobilewright-agent";

    private Object uiAutomation;       // android.app.UiAutomation
    private Method getRootInActiveWindow;
    private Method takeScreenshot;
    private Method waitForIdle;
    private Method setFlags;            // setServiceInfo path not needed; use setFlags

    private Object inputManager;        // android.hardware.input.InputManager
    private Method injectInputEvent;

    public static void main(String[] args) {
        try {
            // The socket name may be passed as the first arg (the host derives a
            // build-specific name so a new agent build supersedes an old one);
            // fall back to the default when launched without it.
            String socketName = (args.length > 0 && !args[0].isEmpty()) ? args[0] : SOCKET_NAME;
            new Agent().run(socketName);
        } catch (Throwable t) {
            System.err.println("agent fatal: " + t);
            t.printStackTrace();
            System.exit(1);
        }
    }

    private void run(String socketName) throws Exception {
        connectUiAutomation();
        connectInputManager();

        LocalServerSocket server = new LocalServerSocket(socketName);
        System.out.println("mobilewright-agent listening on @" + socketName);
        System.out.flush();

        // single-threaded request loop: one client (mobilecli) at a time.
        while (true) {
            LocalSocket client = server.accept();
            try {
                serve(client);
            } catch (Throwable t) {
                System.err.println("client error: " + t);
            } finally {
                try { client.close(); } catch (Throwable ignored) {}
            }
        }
    }

    // ─── Connection setup ────────────────────────────────────────────

    private void connectUiAutomation() throws Exception {
        HandlerThread ht = new HandlerThread("mw-ui");
        ht.start();
        Looper looper = ht.getLooper();

        Class<?> connCls = Class.forName("android.app.UiAutomationConnection");
        Object conn = connCls.getDeclaredConstructor().newInstance();

        Class<?> iConnCls = Class.forName("android.app.IUiAutomationConnection");
        Class<?> uaCls = Class.forName("android.app.UiAutomation");
        Constructor<?> ctor = uaCls.getConstructor(Looper.class, iConnCls);
        uiAutomation = ctor.newInstance(looper, conn);

        Method connect;
        try {
            // newer API: connect(int flags) — flags 0 means no a11y-tools default set
            connect = uaCls.getMethod("connect", int.class);
            connect.invoke(uiAutomation, 0);
        } catch (NoSuchMethodException e) {
            connect = uaCls.getMethod("connect");
            connect.invoke(uiAutomation);
        }

        getRootInActiveWindow = uaCls.getMethod("getRootInActiveWindow");
        try {
            takeScreenshot = uaCls.getMethod("takeScreenshot");
        } catch (NoSuchMethodException e) {
            takeScreenshot = null;
        }
        try {
            waitForIdle = uaCls.getMethod("waitForIdle", long.class, long.class);
        } catch (NoSuchMethodException e) {
            waitForIdle = null;
        }

        System.out.println("UiAutomation connected");
    }

    private void connectInputManager() throws Exception {
        Class<?> imCls = Class.forName("android.hardware.input.InputManager");
        Method getInstance;
        try {
            getInstance = imCls.getMethod("getInstance");
            inputManager = getInstance.invoke(null);
        } catch (NoSuchMethodException e) {
            // API 33+: InputManagerGlobal
            Class<?> img = Class.forName("android.hardware.input.InputManagerGlobal");
            Object global = img.getMethod("getInstance").invoke(null);
            inputManager = global;
        }
        injectInputEvent = inputManager.getClass().getMethod(
                "injectInputEvent", InputEvent.class, int.class);
        injectInputEvent.setAccessible(true);
        System.out.println("InputManager ready");
    }

    // ─── Request loop ────────────────────────────────────────────────

    private void serve(LocalSocket client) throws Exception {
        InputStream in = client.getInputStream();
        OutputStream out = client.getOutputStream();
        StringBuilder line = new StringBuilder();
        int b;
        while ((b = in.read()) != -1) {
            if (b == '\n') {
                String req = line.toString();
                line.setLength(0);
                if (req.trim().isEmpty()) continue;
                String resp = dispatch(req);
                out.write(resp.getBytes("UTF-8"));
                out.write('\n');
                out.flush();
            } else {
                line.append((char) b);
            }
        }
    }

    /** Minimal command protocol: "<id> <method> <arg1> <arg2> ..." → JSON line. */
    private String dispatch(String req) {
        String[] parts = req.trim().split("\\s+");
        String id = parts[0];
        String method = parts.length > 1 ? parts[1] : "";
        try {
            switch (method) {
                case "ping":
                    return ok(id, "\"pong\"");
                case "tap": {
                    int x = Integer.parseInt(parts[2]);
                    int y = Integer.parseInt(parts[3]);
                    tap(x, y);
                    return ok(id, "true");
                }
                case "swipe": {
                    int x1 = Integer.parseInt(parts[2]);
                    int y1 = Integer.parseInt(parts[3]);
                    int x2 = Integer.parseInt(parts[4]);
                    int y2 = Integer.parseInt(parts[5]);
                    int dur = parts.length > 6 ? Integer.parseInt(parts[6]) : 300;
                    swipe(x1, y1, x2, y2, dur);
                    return ok(id, "true");
                }
                case "longpress": {
                    int x = Integer.parseInt(parts[2]);
                    int y = Integer.parseInt(parts[3]);
                    int dur = parts.length > 4 ? Integer.parseInt(parts[4]) : 600;
                    longPress(x, y, dur);
                    return ok(id, "true");
                }
                case "key": {
                    // accepts a keycode name (e.g. KEYCODE_HOME) or a raw int
                    key(resolveKeyCode(parts[2]));
                    return ok(id, "true");
                }
                case "text": {
                    // payload is base64(UTF-8) so spaces/unicode survive the
                    // whitespace-delimited protocol
                    String decoded = new String(
                            android.util.Base64.decode(parts[2], android.util.Base64.NO_WRAP), "UTF-8");
                    if (!typeText(decoded)) {
                        return err(id, "unsupported-characters"); // caller falls back to adb/clipboard
                    }
                    return ok(id, "true");
                }
                case "gesture": {
                    String json = new String(
                            android.util.Base64.decode(parts[2], android.util.Base64.NO_WRAP), "UTF-8");
                    gesture(json);
                    return ok(id, "true");
                }
                case "dump":
                    return ok(id, dumpHierarchy());
                case "screenshot": {
                    String format = parts.length > 2 ? parts[2] : "png";
                    int quality = parts.length > 3 ? Integer.parseInt(parts[3]) : 90;
                    String b64 = screenshot(format, quality);
                    return ok(id, "{\"data\":\"" + b64 + "\"}");
                }
                default:
                    return err(id, "unknown method: " + method);
            }
        } catch (Throwable t) {
            return err(id, String.valueOf(t));
        }
    }

    // ─── Input ───────────────────────────────────────────────────────

    private static final int INJECT_ASYNC = 0;

    private void injectMotion(int action, float x, float y, long downTime) throws Exception {
        long now = SystemClock.uptimeMillis();
        MotionEvent ev = MotionEvent.obtain(downTime, now, action, x, y, 0);
        ev.setSource(0x00001002); // SOURCE_TOUCHSCREEN
        injectInputEvent.invoke(inputManager, ev, INJECT_ASYNC);
        ev.recycle();
    }

    private void tap(int x, int y) throws Exception {
        long down = SystemClock.uptimeMillis();
        injectMotion(MotionEvent.ACTION_DOWN, x, y, down);
        injectMotion(MotionEvent.ACTION_UP, x, y, down);
    }

    private void swipe(int x1, int y1, int x2, int y2, int durationMs) throws Exception {
        long down = SystemClock.uptimeMillis();
        injectMotion(MotionEvent.ACTION_DOWN, x1, y1, down);
        int steps = Math.max(2, durationMs / 10);
        for (int i = 1; i <= steps; i++) {
            float t = (float) i / steps;
            float x = x1 + (x2 - x1) * t;
            float y = y1 + (y2 - y1) * t;
            injectMotion(MotionEvent.ACTION_MOVE, x, y, down);
            SystemClock.sleep(durationMs / steps);
        }
        injectMotion(MotionEvent.ACTION_UP, x2, y2, down);
    }

    private void longPress(int x, int y, int durationMs) throws Exception {
        long down = SystemClock.uptimeMillis();
        injectMotion(MotionEvent.ACTION_DOWN, x, y, down);
        SystemClock.sleep(durationMs);
        injectMotion(MotionEvent.ACTION_UP, x, y, down);
    }

    private static int resolveKeyCode(String token) {
        try {
            return Integer.parseInt(token);
        } catch (NumberFormatException e) {
            int code = KeyEvent.keyCodeFromString(token);
            if (code == KeyEvent.KEYCODE_UNKNOWN) {
                throw new IllegalArgumentException("unknown keycode: " + token);
            }
            return code;
        }
    }

    private void key(int keyCode) throws Exception {
        long now = SystemClock.uptimeMillis();
        KeyEvent down = new KeyEvent(now, now, KeyEvent.ACTION_DOWN, keyCode, 0);
        injectInputEvent.invoke(inputManager, down, INJECT_ASYNC);
        KeyEvent up = new KeyEvent(now, now, KeyEvent.ACTION_UP, keyCode, 0);
        injectInputEvent.invoke(inputManager, up, INJECT_ASYNC);
    }

    /**
     * Types text the same way `adb shell input text` does — via the virtual
     * KeyCharacterMap — but without the per-call JVM spawn. Returns false if the
     * char map cannot produce events for the input (e.g. non-ASCII / emoji), so
     * the caller can fall back to the clipboard path.
     */
    private boolean typeText(String text) throws Exception {
        if (text.isEmpty()) return true;
        android.view.KeyCharacterMap kcm =
                android.view.KeyCharacterMap.load(android.view.KeyCharacterMap.VIRTUAL_KEYBOARD);
        KeyEvent[] events = kcm.getEvents(text.toCharArray());
        if (events == null) return false;
        for (KeyEvent ev : events) {
            injectInputEvent.invoke(inputManager, ev, INJECT_ASYNC);
        }
        return true;
    }

    /**
     * Executes a pointer gesture described as a JSON array of actions:
     * [{"type":"pointerDown|pointerMove|pointerUp|pause","x":..,"y":..,"duration":..}]
     * mirroring mobilecli's wda.TapAction. A single downTime ties the stream
     * together so it registers as one continuous gesture.
     */
    private void gesture(String json) throws Exception {
        java.util.List<float[]> ops = parseGesture(json); // [kind, x, y, duration]
        long down = SystemClock.uptimeMillis();
        float x = 0, y = 0;
        for (float[] op : ops) {
            int kind = (int) op[0];
            switch (kind) {
                case GESTURE_DOWN:
                    x = op[1]; y = op[2];
                    injectMotion(MotionEvent.ACTION_DOWN, x, y, down);
                    break;
                case GESTURE_MOVE:
                    x = op[1]; y = op[2];
                    injectMotion(MotionEvent.ACTION_MOVE, x, y, down);
                    break;
                case GESTURE_UP:
                    injectMotion(MotionEvent.ACTION_UP, x, y, down);
                    break;
                case GESTURE_PAUSE:
                    SystemClock.sleep((long) op[3]);
                    break;
            }
        }
    }

    private static final int GESTURE_DOWN = 0, GESTURE_MOVE = 1, GESTURE_UP = 2, GESTURE_PAUSE = 3;

    /** Minimal hand-rolled parser for the gesture JSON (no JSON lib on bootclasspath). */
    private static java.util.List<float[]> parseGesture(String json) {
        java.util.List<float[]> out = new java.util.ArrayList<>();
        java.util.regex.Matcher obj = java.util.regex.Pattern
                .compile("\\{[^}]*\\}").matcher(json);
        while (obj.find()) {
            String o = obj.group();
            String type = extract(o, "type");
            int kind;
            switch (type) {
                case "pointerDown": kind = GESTURE_DOWN; break;
                case "pointerMove": kind = GESTURE_MOVE; break;
                case "pointerUp":   kind = GESTURE_UP;   break;
                case "pause":       kind = GESTURE_PAUSE; break;
                default: continue;
            }
            out.add(new float[]{
                    kind,
                    numField(o, "x"),
                    numField(o, "y"),
                    numField(o, "duration"),
            });
        }
        return out;
    }

    private static String extract(String obj, String key) {
        java.util.regex.Matcher m = java.util.regex.Pattern
                .compile("\"" + key + "\"\\s*:\\s*\"([^\"]*)\"").matcher(obj);
        return m.find() ? m.group(1) : "";
    }

    private static float numField(String obj, String key) {
        java.util.regex.Matcher m = java.util.regex.Pattern
                .compile("\"" + key + "\"\\s*:\\s*(-?\\d+(?:\\.\\d+)?)").matcher(obj);
        return m.find() ? Float.parseFloat(m.group(1)) : 0f;
    }

    // ─── Hierarchy ─────────────────────────────────────────────────────

    private String dumpHierarchy() throws Exception {
        // getRootInActiveWindow can transiently return null during window
        // transitions; retry briefly rather than emit an empty tree.
        AccessibilityNodeInfo root = null;
        for (int i = 0; i < 5; i++) {
            root = (AccessibilityNodeInfo) getRootInActiveWindow.invoke(uiAutomation);
            if (root != null) break;
            SystemClock.sleep(50);
        }
        StringBuilder sb = new StringBuilder();
        sb.append("{\"elements\":[");
        if (root != null) {
            serializeNode(root, sb, true);
        }
        sb.append("]}");
        return sb.toString();
    }

    // recycle() is deprecated and a no-op on API 33+, but this agent is built
    // with min-api 24 and runs long-lived with frequent dumps; recycling each
    // node post-traversal returns it to the pool on API 24-32 to avoid
    // allocation churn there. Harmless elsewhere.
    @SuppressWarnings("deprecation")
    private void serializeNode(AccessibilityNodeInfo node, StringBuilder sb, boolean first) {
        if (node == null) return;
        if (!first) sb.append(',');

        try {
            android.graphics.Rect r = new android.graphics.Rect();
            node.getBoundsInScreen(r);

            sb.append('{');
            sb.append("\"type\":").append(jstr(text(node.getClassName())));
            sb.append(",\"text\":").append(jstr(text(node.getText())));
            sb.append(",\"label\":").append(jstr(text(node.getContentDescription())));
            sb.append(",\"identifier\":").append(jstr(text(node.getViewIdResourceName())));
            sb.append(",\"enabled\":").append(node.isEnabled());
            sb.append(",\"visible\":").append(node.isVisibleToUser());
            sb.append(",\"rect\":{\"x\":").append(r.left)
              .append(",\"y\":").append(r.top)
              .append(",\"width\":").append(r.width())
              .append(",\"height\":").append(r.height()).append('}');

            int n = node.getChildCount();
            sb.append(",\"children\":[");
            boolean childFirst = true;
            for (int i = 0; i < n; i++) {
                AccessibilityNodeInfo child = node.getChild(i);
                if (child != null) {
                    serializeNode(child, sb, childFirst);
                    childFirst = false;
                }
            }
            sb.append("]}");
        } finally {
            // post-order: node is fully consumed and not touched after this
            node.recycle();
        }
    }

    // ─── Screenshot ────────────────────────────────────────────────────

    private String screenshot(String format, int quality) throws Exception {
        if (takeScreenshot == null) throw new IllegalStateException("takeScreenshot unavailable");
        Bitmap bmp = (Bitmap) takeScreenshot.invoke(uiAutomation);
        if (bmp == null) throw new IllegalStateException("takeScreenshot returned null");
        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        Bitmap.CompressFormat fmt = "jpeg".equalsIgnoreCase(format) || "jpg".equalsIgnoreCase(format)
                ? Bitmap.CompressFormat.JPEG
                : Bitmap.CompressFormat.PNG;
        bmp.compress(fmt, quality, baos);
        bmp.recycle();
        return android.util.Base64.encodeToString(baos.toByteArray(), android.util.Base64.NO_WRAP);
    }

    // ─── JSON helpers ──────────────────────────────────────────────────

    private static String text(CharSequence cs) {
        return cs == null ? "" : cs.toString();
    }

    private static String jstr(String s) {
        StringBuilder sb = new StringBuilder("\"");
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            switch (c) {
                case '"':  sb.append("\\\""); break;
                case '\\': sb.append("\\\\"); break;
                case '\n': sb.append("\\n"); break;
                case '\r': sb.append("\\r"); break;
                case '\t': sb.append("\\t"); break;
                default:
                    if (c < 0x20) sb.append(String.format("\\u%04x", (int) c));
                    else sb.append(c);
            }
        }
        return sb.append('"').toString();
    }

    private static String ok(String id, String result) {
        return "{\"id\":" + id + ",\"result\":" + result + "}";
    }

    private static String err(String id, String message) {
        return "{\"id\":" + id + ",\"error\":" + jstr(message) + "}";
    }
}
