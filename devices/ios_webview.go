package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	goios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/debuggertools"
	"github.com/danielpaulus/go-ios/ios/debugserver"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/mobile-next/mobilecli/agents"
	iosutil "github.com/mobile-next/mobilecli/devices/ios"
	"github.com/mobile-next/mobilecli/utils"
)

// agentPortCache maps device UDID → local TCP port of its injected agent.
var (
	agentPortCache   = map[string]int{}
	agentPortCacheMu sync.Mutex
)

func cachedAgentPort(udid string) (int, bool) {
	agentPortCacheMu.Lock()
	defer agentPortCacheMu.Unlock()
	port, ok := agentPortCache[udid]
	return port, ok
}

func setCachedAgentPort(udid string, port int) {
	agentPortCacheMu.Lock()
	defer agentPortCacheMu.Unlock()
	agentPortCache[udid] = port
}

// findSimulatorForegroundApp searches the Mac process list for an app process
// running inside the given simulator and returns its PID and bundle ID.
func findSimulatorForegroundApp(udid string) (pid int, bundleID string, err error) {
	out, err := exec.Command("ps", "aux").Output()
	if err != nil {
		return 0, "", fmt.Errorf("ps aux: %w", err)
	}

	// match lines for app binaries inside this simulator's Bundle directory
	pattern := fmt.Sprintf(`CoreSimulator/Devices/%s/data/Containers/Bundle`, udid)
	var candidates []struct {
		pid  int
		path string
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, pattern) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		p, parseErr := strconv.Atoi(fields[1])
		if parseErr != nil {
			continue
		}
		// fields[10] is the executable path
		candidates = append(candidates, struct {
			pid  int
			path string
		}{p, fields[10]})
	}

	if len(candidates) == 0 {
		return 0, "", fmt.Errorf("no app process found in simulator %s — is an app running?", udid)
	}
	if len(candidates) > 1 {
		// pick the most recently listed (last in ps output) as a heuristic for foreground
		// in practice most simulator sessions have a single app
	}
	candidate := candidates[len(candidates)-1]

	// extract the .app bundle directory from the executable path
	appBundlePath := appBundleFromExecPath(candidate.path)
	if appBundlePath == "" {
		return 0, "", fmt.Errorf("could not locate .app bundle from path %q", candidate.path)
	}

	bid, err := bundleIDFromInfoPlist(filepath.Join(appBundlePath, "Info.plist"))
	if err != nil {
		return 0, "", fmt.Errorf("read bundle ID: %w", err)
	}

	return candidate.pid, bid, nil
}

// appBundleFromExecPath returns the .app directory containing the executable.
// e.g. ".../playground.app/playground" → ".../playground.app"
func appBundleFromExecPath(execPath string) string {
	re := regexp.MustCompile(`(.*\.app)/`)
	m := re.FindStringSubmatch(execPath)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// bundleIDFromInfoPlist reads CFBundleIdentifier from an Info.plist using
// the macOS `defaults read` command, which handles both XML and binary plists.
func bundleIDFromInfoPlist(plistPath string) (string, error) {
	out, err := exec.Command("defaults", "read", plistPath, "CFBundleIdentifier").Output()
	if err != nil {
		return "", fmt.Errorf("defaults read %s: %w", plistPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// writeIOSAgentDylib writes the embedded simulator dylib to a temp file and
// returns the path. The caller is responsible for removing the file.
func writeIOSAgentDylib() (string, error) {
	f, err := os.CreateTemp("", "mobilecli-agent-*.dylib")
	if err != nil {
		return "", fmt.Errorf("create temp dylib: %w", err)
	}
	if _, err := f.Write(agents.IOSAgentSimDylib); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write dylib: %w", err)
	}
	f.Close()
	return f.Name(), nil
}

var portFromLLDB = regexp.MustCompile(`\$\d+\s*=\s*(\d+)`)

// injectIOSAgent attaches lldb to the given PID, loads the dylib, reads the
// bound port via mobilecli_get_port(), then detaches. Returns the port.
func injectIOSAgent(pid int, dylibPath string) (int, error) {
	cmd := exec.Command("lldb",
		"-p", strconv.Itoa(pid),
		"-o", fmt.Sprintf("expr (void*)dlopen(%q, 2)", dylibPath),
		"-o", "expr (int)mobilecli_get_port()",
		"-o", "detach",
		"-o", "quit",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("lldb: %w\noutput:\n%s", err, out)
	}

	// find the port in a line like: (int) $1 = 27042
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "mobilecli_get_port") {
			continue
		}
		m := portFromLLDB.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err != nil || port == 0 {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("could not parse port from lldb output:\n%s", out)
}

// ensureIOSAgentReady ensures the iOS agent is running inside the simulator
// and returns the local TCP port to connect to.
func (s *SimulatorDevice) ensureIOSAgentReady() (int, error) {
	// fast path: reuse the port we injected into this simulator previously
	if port, ok := cachedAgentPort(s.UDID); ok && isAgentReady(port) {
		return port, nil
	}

	pid, _, err := findSimulatorForegroundApp(s.UDID)
	if err != nil {
		return 0, err
	}

	dylibPath, err := writeIOSAgentDylib()
	if err != nil {
		return 0, err
	}
	defer os.Remove(dylibPath)

	port, err := injectIOSAgent(pid, dylibPath)
	if err != nil {
		return 0, fmt.Errorf("inject agent: %w", err)
	}

	// agent binds synchronously before dlopen returns, but give it a moment
	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(port) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("iOS agent did not respond on port %d within 3s", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
	setCachedAgentPort(s.UDID, port)
	return port, nil
}

func (s *SimulatorDevice) webViewAction(wvID, method string) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, method, map[string]any{"id": wvID})
	return err
}

// WebViewReload reloads the page in the given webview.
func (s *SimulatorDevice) WebViewReload(wvID string) error {
	return s.webViewAction(wvID, "device.webview.reload")
}

// WebViewGoBack navigates the given webview back in history.
func (s *SimulatorDevice) WebViewGoBack(wvID string) error {
	return s.webViewAction(wvID, "device.webview.goBack")
}

// WebViewGoForward navigates the given webview forward in history.
func (s *SimulatorDevice) WebViewGoForward(wvID string) error {
	return s.webViewAction(wvID, "device.webview.goForward")
}

// WebViewContent returns the full outer HTML of the page in the given webview.
func (s *SimulatorDevice) WebViewContent(wvID string) (string, error) {
	result, err := s.WebViewEvaluate(wvID, "return document.documentElement.outerHTML", nil)
	if err != nil {
		return "", err
	}
	content, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T", result)
	}
	return content, nil
}

// WebViewWaitForLoadState blocks until the webview reaches the given load state.
// timeoutMs of 0 uses the agent's default (30s).
func (s *SimulatorDevice) WebViewWaitForLoadState(wvID, state string, timeoutMs int) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	const agentDefaultMs = 30_000
	waitMs := agentDefaultMs
	if timeoutMs > 0 {
		waitMs = timeoutMs
	}
	params := map[string]any{"id": wvID, "timeout": waitMs}
	if state != "" {
		params["state"] = state
	}
	httpTimeout := time.Duration(waitMs)*time.Millisecond + 5*time.Second
	_, err = agentRequestWithTimeout(port, "device.webview.waitForLoadState", params, httpTimeout)
	return err
}

// WebViewGoto navigates the webview identified by wvID to url.
func (s *SimulatorDevice) WebViewGoto(wvID, url string) error {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return err
	}
	_, err = agentRequest(port, "device.webview.goto", map[string]any{"id": wvID, "url": url})
	return err
}

// WebViewEvaluate runs expression in the webview and returns the JS result value.
func (s *SimulatorDevice) WebViewEvaluate(wvID, expression string, args []any) (any, error) {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"id":         wvID,
		"expression": ensureReturnExpression(expression),
	}
	if len(args) > 0 {
		params["args"] = args
	}
	raw, err := agentRequest(port, "device.webview.evaluate", params)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Result any `json:"result"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("parse evaluate result: %w", err)
	}
	return wrapper.Result, nil
}

// ListWebViews returns all embedded WKWebViews found in the foreground simulator app.
func (s *SimulatorDevice) ListWebViews() ([]WebViewInfo, error) {
	port, err := s.ensureIOSAgentReady()
	if err != nil {
		return nil, err
	}

	result, err := agentRequest(port, "device.webview.list", nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID      string         `json:"id"`
		URL     string         `json:"url"`
		Title   string         `json:"title"`
		Bounds  map[string]any `json:"bounds"`
		Visible bool           `json:"visible"`
	}
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, fmt.Errorf("parse webview list: %w", err)
	}

	webviews := make([]WebViewInfo, len(raw))
	for i, wv := range raw {
		webviews[i] = WebViewInfo{
			ID:        wv.ID,
			URL:       wv.URL,
			Title:     wv.Title,
			Bounds:    wv.Bounds,
			IsVisible: wv.Visible,
		}
	}
	return webviews, nil
}

// ── IOSDevice (real device) ───────────────────────────────────────────────────


type userApp struct {
	pid      int
	bundleID string
	teamID   string
}

// userApps returns all currently running user-installed apps (PID + bundle ID),
// using installationproxy + instruments, with no WDA dependency.
func (d *IOSDevice) userApps(device goios.DeviceEntry) ([]userApp, error) {
	utils.Verbose("connecting to installationproxy")
	svc, err := installationproxy.New(device)
	if err != nil {
		return nil, fmt.Errorf("installationproxy: %w", err)
	}
	defer svc.Close()

	utils.Verbose("browsing user apps")
	apps, err := svc.BrowseUserApps()
	if err != nil {
		return nil, fmt.Errorf("browse user apps: %w", err)
	}
	utils.Verbose("found %d installed user apps", len(apps))
	execToBundleID := map[string]string{}
	for _, app := range apps {
		execToBundleID[app.CFBundleExecutable()] = app.CFBundleIdentifier()
	}

	utils.Verbose("connecting to instruments device info service")
	infoSvc, err := instruments.NewDeviceInfoService(device)
	if err != nil {
		return nil, fmt.Errorf("device info service: %w", err)
	}
	defer infoSvc.Close()

	utils.Verbose("fetching process list")
	processes, err := infoSvc.ProcessList()
	if err != nil {
		return nil, fmt.Errorf("process list: %w", err)
	}
	utils.Verbose("got %d processes", len(processes))

	// also build a map from bundleID to teamIdentifier
	bundleToTeam := map[string]string{}
	for _, app := range apps {
		if tid, ok := app["TeamIdentifier"].(string); ok {
			bundleToTeam[app.CFBundleIdentifier()] = tid
		}
	}

	var result []userApp
	for _, p := range processes {
		if bid, ok := execToBundleID[p.Name]; ok {
			result = append(result, userApp{pid: int(p.Pid), bundleID: bid, teamID: bundleToTeam[bid]})
		}
	}
	utils.Verbose("found %d running user apps", len(result))
	return result, nil
}

// findForegroundApp finds the foreground user app by attaching to each candidate
// via the CoreDevice debug proxy and checking UIApplicationState via ObjC runtime.
func (d *IOSDevice) findForegroundApp(device goios.DeviceEntry, apps []userApp) (*userApp, error) {
	if !device.SupportsRsd() {
		return nil, fmt.Errorf("device does not support RSD")
	}
	proxyPort := device.Rsd.GetPort("com.apple.internal.dt.remote.debugproxy")
	if proxyPort == 0 {
		return nil, fmt.Errorf("com.apple.internal.dt.remote.debugproxy not in RSD")
	}
	utils.Verbose("debug proxy port: %d", proxyPort)

	for i := range apps {
		app := &apps[i]
		utils.Verbose("checking app %s (pid %d)", app.bundleID, app.pid)
		conn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
		if err != nil {
			utils.Verbose("connect to debug proxy for %s: %v", app.bundleID, err)
			continue
		}
		gdb := debugserver.NewGDBServer(conn)
		utils.Verbose("attaching to pid %d", app.pid)
		resp, err := gdb.Request(fmt.Sprintf("vAttach;%x", app.pid))
		if err != nil || !strings.HasPrefix(resp, "T") {
			utils.Verbose("attach to pid %d failed: err=%v resp=%q", app.pid, err, resp)
			conn.Close()
			continue
		}
		utils.Verbose("attached to pid %d, checking UIApplicationState", app.pid)
		rt, err := debuggertools.NewObjCRuntime(gdb)
		if err != nil {
			utils.Verbose("ObjCRuntime for pid %d: %v", app.pid, err)
			gdb.Request(fmt.Sprintf("D;%x", app.pid)) //nolint:errcheck
			conn.Close()
			continue
		}
		appInst, err := rt.ClassCall("UIApplication", "sharedApplication")
		var state uint64
		if err == nil {
			state, _ = rt.Call(appInst, "applicationState")
		}
		rt.Cleanup()
		gdb.Request(fmt.Sprintf("D;%x", app.pid)) //nolint:errcheck
		conn.Close()
		utils.Verbose("pid %d (%s) applicationState=%d", app.pid, app.bundleID, state)
		if err == nil && state == 0 {
			utils.Verbose("foreground app: %s (pid %d)", app.bundleID, app.pid)
			return app, nil
		}
	}
	return nil, fmt.Errorf("no foreground user app found — is an app open?")
}

// iosDeviceAgentExpr is an ObjC expression evaluated via LLDB that binds a TCP
// server socket inside the target app process and starts an HTTP/JSON-RPC
// accept loop on a GCD background queue. The accept loop persists after LLDB
// detaches. The expression evaluates to the bound port number.
const iosDeviceAgentExpr = `
@import Foundation; @import UIKit; @import WebKit;
// inline C declarations — LLDB remote-ios doesn't have SDK headers on include path
typedef unsigned int __socklen_t;
typedef unsigned short __in_port_t;
typedef unsigned int __in_addr_t;
struct __in_addr { __in_addr_t s_addr; };
struct __sockaddr_in {
    unsigned char sin_len; unsigned char sin_family;
    __in_port_t sin_port; struct __in_addr sin_addr; char sin_zero[8];
};
struct __sockaddr { unsigned char sa_len; unsigned char sa_family; char sa_data[14]; };
extern int socket(int, int, int);
extern int setsockopt(int, int, int, const void *, __socklen_t);
extern int bind(int, const struct __sockaddr *, __socklen_t);
extern int listen(int, int);
extern int accept(int, struct __sockaddr *, __socklen_t *);
extern long recv(int, void *, unsigned long, int);
extern long send(int, const void *, unsigned long, int);
extern int close(int);
extern __in_port_t htons(__in_port_t);
extern __in_addr_t htonl(__in_addr_t);
extern void *memset(void *, int, unsigned long);
#define __AF_INET    2
#define __SOCK_STREAM 1
#define __SOL_SOCKET  0xffff
#define __SO_REUSEADDR 0x0004
#define __INADDR_LOOPBACK 0x7f000001UL
int __sfd = socket(__AF_INET, __SOCK_STREAM, 0);
int __opt = 1;
setsockopt(__sfd, __SOL_SOCKET, __SO_REUSEADDR, &__opt, sizeof(__opt));
struct __sockaddr_in __sa; memset(&__sa, 0, sizeof(__sa));
__sa.sin_family = __AF_INET;
__sa.sin_addr.s_addr = htonl(__INADDR_LOOPBACK);
int __port = 0;
for (int __p = 27042; __p < 27052; __p++) {
    __sa.sin_port = htons((unsigned short)__p);
    if (bind(__sfd, (struct __sockaddr *)&__sa, sizeof(__sa)) == 0) { __port = __p; break; }
}
if (__port > 0) {
    listen(__sfd, 8);
    int __srv = __sfd;
    dispatch_async(dispatch_get_global_queue(0, 0), ^{
        WKWebView *(^__findWV)(NSString *) = ^WKWebView *(NSString *wvId) {
            __block WKWebView *found = nil;
            id sem = dispatch_semaphore_create(0);
            [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                Class wk = NSClassFromString(@"WKWebView");
                for (UIScene *sc in [UIApplication sharedApplication].connectedScenes)
                    if ([sc isKindOfClass:[UIWindowScene class]])
                        for (UIWindow *w in ((UIWindowScene *)sc).windows) {
                            NSMutableArray *stk = [NSMutableArray arrayWithObject:w];
                            while (stk.count) {
                                UIView *v = stk[0]; [stk removeObjectAtIndex:0];
                                if (wk && [v isKindOfClass:wk] &&
                                    [[NSString stringWithFormat:@"%p", v] isEqualToString:wvId])
                                { found = (WKWebView *)v; break; }
                                [stk addObjectsFromArray:v.subviews];
                            }
                            if (found) break;
                        }
                dispatch_semaphore_signal(sem);
            }];
            dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
            return found;
        };
        while (1) {
            int cfd = accept(__srv, NULL, NULL);
            if (cfd < 0) continue;
            dispatch_async(dispatch_get_global_queue(0, 0), ^{
                NSMutableData *buf = [NSMutableData data];
                char tmp[4096]; long n;
                NSData *crlf = [@"\r\n\r\n" dataUsingEncoding:NSASCIIStringEncoding];
                while ((n = recv(cfd, tmp, sizeof(tmp), 0)) > 0) {
                    [buf appendBytes:tmp length:(NSUInteger)n];
                    NSRange sep = [buf rangeOfData:crlf options:0 range:NSMakeRange(0, buf.length)];
                    if (sep.location != NSNotFound) {
                        NSString *hdr = [[NSString alloc] initWithData:[buf subdataWithRange:NSMakeRange(0, sep.location)] encoding:NSASCIIStringEncoding];
                        NSInteger cl = 0;
                        for (NSString *__hdrLine in [hdr componentsSeparatedByString:@"\r\n"])
                            if ([__hdrLine.lowercaseString hasPrefix:@"content-length:"])
                                cl = [[__hdrLine substringFromIndex:15] integerValue];
                        NSUInteger bs = sep.location + 4;
                        while ((NSInteger)(buf.length - bs) < cl && (n = recv(cfd, tmp, sizeof(tmp), 0)) > 0)
                            [buf appendBytes:tmp length:(NSUInteger)n];
                        break;
                    }
                }
                NSRange hr = [buf rangeOfData:crlf options:0 range:NSMakeRange(0, buf.length)];
                NSData *body = (hr.location == NSNotFound) ? [NSData data] :
                    [buf subdataWithRange:NSMakeRange(hr.location + 4, buf.length - hr.location - 4)];
                NSDictionary *req = [NSJSONSerialization JSONObjectWithData:body options:0 error:nil];
                id rqId = req[@"id"] ?: [NSNull null];
                NSString *method = req[@"method"] ?: @"";
                NSDictionary *params = req[@"params"];
                NSData *resp = nil;
                if ([method isEqualToString:@"device.webview.list"]) {
                    __block NSMutableArray *wvs = [NSMutableArray array];
                    id sem = dispatch_semaphore_create(0);
                    [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                        Class wk = NSClassFromString(@"WKWebView");
                        for (UIScene *sc in [UIApplication sharedApplication].connectedScenes)
                            if ([sc isKindOfClass:[UIWindowScene class]])
                                for (UIWindow *win in ((UIWindowScene *)sc).windows) {
                                    NSMutableArray *stk = [NSMutableArray arrayWithObject:win];
                                    while (stk.count) {
                                        UIView *v = stk[0]; [stk removeObjectAtIndex:0];
                                        if (wk && [v isKindOfClass:wk]) {
                                            WKWebView *wv = (WKWebView *)v;
                                            [wvs addObject:@{
                                                @"id": [NSString stringWithFormat:@"%p", wv],
                                                @"url": wv.URL.absoluteString ?: @"",
                                                @"title": wv.title ?: @"",
                                                @"bounds": @{@"x":@0,@"y":@0,@"width":@0,@"height":@0},
                                                @"visible": @(!wv.isHidden && wv.window != nil)
                                            }];
                                        }
                                        [stk addObjectsFromArray:v.subviews];
                                    }
                                }
                        dispatch_semaphore_signal(sem);
                    }];
                    dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                    resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":wvs} options:0 error:nil];
                } else if ([method isEqualToString:@"device.webview.goto"]) {
                    NSString *wvId = params[@"id"], *url = params[@"url"];
                    WKWebView *wv = (wvId && url) ? __findWV(wvId) : nil;
                    if (!wvId || !url) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id or url"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{ [wv loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:url]]]; dispatch_semaphore_signal(sem); }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                        resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"status":@"ok"}} options:0 error:nil];
                    }
                } else if ([@[@"device.webview.reload",@"device.webview.goBack",@"device.webview.goForward"] containsObject:method]) {
                    NSString *wvId = params[@"id"];
                    WKWebView *wv = wvId ? __findWV(wvId) : nil;
                    if (!wvId) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                            if ([method isEqualToString:@"device.webview.reload"]) [wv reload];
                            else if ([method isEqualToString:@"device.webview.goBack"]) [wv goBack];
                            else [wv goForward];
                            dispatch_semaphore_signal(sem);
                        }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                        resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"status":@"ok"}} options:0 error:nil];
                    }
                } else if ([method isEqualToString:@"device.webview.evaluate"]) {
                    NSString *wvId = params[@"id"], *expr = params[@"expression"];
                    WKWebView *wv = (wvId && expr) ? __findWV(wvId) : nil;
                    if (!wvId || !expr) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id or expression"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        NSString *wrapped = [NSString stringWithFormat:@"(function(){try{%@}catch(e){return{__mce:e.toString()}}})()", expr];
                        __block id jsResult = nil; __block NSError *jsError = nil;
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                            [wv evaluateJavaScript:wrapped completionHandler:^(id r, NSError *e) { jsResult = r; jsError = e; dispatch_semaphore_signal(sem); }];
                        }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 10000000000LL));
                        if (jsError) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":jsError.localizedDescription}} options:0 error:nil];
                        else if ([(NSObject *)jsResult isKindOfClass:[NSDictionary class]] && jsResult[@"__mce"]) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":jsResult[@"__mce"]}} options:0 error:nil];
                        else resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"result":jsResult?:[NSNull null]}} options:0 error:nil];
                    }
                } else if ([method isEqualToString:@"device.webview.waitForLoadState"]) {
                    NSString *wvId = params[@"id"];
                    WKWebView *wv = wvId ? __findWV(wvId) : nil;
                    if (!wvId) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        NSString *state = params[@"state"] ?: @"load";
                        NSInteger toMs = params[@"timeout"] ? [(NSNumber *)params[@"timeout"] integerValue] : 30000;
                        NSString *checkJS = [@"domcontentloaded" isEqualToString:state] ?
                            @"return String(document.readyState==='interactive'||document.readyState==='complete')" :
                            @"return String(document.readyState==='complete')";
                        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:toMs / 1000.0];
                        BOOL done = NO;
                        while (!done && [[NSDate date] compare:deadline] == NSOrderedAscending) {
                            __block id jsR = nil;
                            id sem2 = dispatch_semaphore_create(0);
                            NSString *wrapped = [NSString stringWithFormat:@"(function(){try{%@}catch(e){return null}})()", checkJS];
                            [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                                [wv evaluateJavaScript:wrapped completionHandler:^(id r, NSError *e) { jsR = r; dispatch_semaphore_signal(sem2); }];
                            }];
                            dispatch_semaphore_wait(sem2, dispatch_time(0, 5000000000LL));
                            if ([@"true" isEqualToString:jsR]) done = YES;
                            else [NSThread sleepForTimeInterval:0.2];
                        }
                        resp = done ?
                            [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"status":@"ok"}} options:0 error:nil] :
                            [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"timed out"}} options:0 error:nil];
                    }
                }
                if (!resp) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32601),@"message":[NSString stringWithFormat:@"method not found: %@", method]}} options:0 error:nil];
                if (!resp) resp = [@"{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32603,\"message\":\"internal error\"}}" dataUsingEncoding:NSUTF8StringEncoding];
                NSString *hdrs = [NSString stringWithFormat:@"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %lu\r\nConnection: close\r\n\r\n", (unsigned long)resp.length];
                NSData *hdrData = [hdrs dataUsingEncoding:NSASCIIStringEncoding];
                send(cfd, hdrData.bytes, hdrData.length, 0);
                send(cfd, resp.bytes, resp.length, 0);
                close(cfd);
            });
        }
    });
}
(int)__port
`

// startLLDBProxy pre-attaches to pid on the device via the debug proxy, then
// starts a local TCP listener for LLDB. Pre-attaching before listening ensures
// the proxy can respond to LLDB's handshake immediately upon connection.
// Returns the local port and a stop function.
func startLLDBProxy(device goios.DeviceEntry, proxyPort, pid int) (int, func(), error) {
	utils.Verbose("lldb-proxy: connecting to device debug proxy port %d", proxyPort)
	devConn, err := goios.ConnectTUNDevice(device.Address, proxyPort, device)
	if err != nil {
		return 0, nil, fmt.Errorf("lldb-proxy: connect to device: %w", err)
	}

	devGDB := debugserver.NewGDBServer(devConn)
	utils.Verbose("lldb-proxy: pre-attaching to pid %d", pid)
	stopReply, err := devGDB.Request(fmt.Sprintf("vAttach;%x", pid))
	if err != nil || !strings.HasPrefix(stopReply, "T") {
		devConn.Close()
		return 0, nil, fmt.Errorf("lldb-proxy: vAttach pid %d: err=%v resp=%q", pid, err, stopReply)
	}
	utils.Verbose("lldb-proxy: pre-attached, stop=%q", stopReply)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		devConn.Close()
		return 0, nil, fmt.Errorf("listen for lldb proxy: %w", err)
	}
	localPort := ln.Addr().(*net.TCPAddr).Port
	go func() {
		defer ln.Close()
		defer devConn.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		lldbProxyConn(conn, devGDB, pid)
	}()
	return localPort, func() { ln.Close() }, nil
}

// lldbProxyConn is a GDB RSP bridge between LLDB and an already-attached
// device debugserver. Handles negotiation packets locally, forwards all others
// packet-by-packet with ack-mode translation (LLDB: no-ack; device: ack).
func lldbProxyConn(c net.Conn, devGDB *debugserver.GDBServer, pid int) {
	defer c.Close()

	// debugserver sends '+' immediately upon accepting a connection;
	// LLDB waits for this before sending the first packet.
	c.Write([]byte("+")) //nolint:errcheck

	noAck := false

	gdbChecksum := func(pkt string) byte {
		var sum byte
		for i := 0; i < len(pkt); i++ {
			sum += pkt[i]
		}
		return sum
	}

	sendToLLDB := func(pkt string) {
		ck := gdbChecksum(pkt)
		var s string
		if !noAck {
			s = "+"
		}
		s += fmt.Sprintf("$%s#%02x", pkt, ck)
		c.Write([]byte(s)) //nolint:errcheck
	}

	recvFromLLDB := func() (string, error) {
		buf := make([]byte, 1)
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '$' {
				break
			}
		}
		var pkt strings.Builder
		for {
			if _, err := io.ReadFull(c, buf); err != nil {
				return "", err
			}
			if buf[0] == '#' {
				break
			}
			pkt.WriteByte(buf[0])
		}
		cksumBuf := make([]byte, 2)
		if _, err := io.ReadFull(c, cksumBuf); err != nil {
			return "", err
		}
		return pkt.String(), nil
	}

	for {
		pkt, err := recvFromLLDB()
		if err != nil {
			return
		}
		utils.Verbose("lldb-proxy ← LLDB: %.120s", pkt)

		// switchToNoAck is set by QStartNoAckMode and applied AFTER sendToLLDB
		// so the OK response goes out in ack mode (with '+') as LLDB expects.
		switchToNoAck := false
		var reply string
		switch {
		case pkt == "QStartNoAckMode":
			reply = "OK"
			switchToNoAck = true

		case strings.HasPrefix(pkt, "qSupported"):
			reply = "PacketSize=65536;vContSupported+"

		case pkt == "QThreadSuffixSupported",
			pkt == "QListThreadsInStopReply",
			pkt == "qVAttachOrWaitSupported",
			pkt == "QEnableErrorStrings":
			reply = "OK"

		case strings.HasPrefix(pkt, "vCont?"):
			reply = "vCont;c;C;s;S"

		case pkt == "k":
			// LLDB wants to kill — detach instead so the app keeps running
			devGDB.Request(fmt.Sprintf("D;%x", pid)) //nolint:errcheck
			return

		case strings.HasPrefix(pkt, "D"):
			devReply, _ := devGDB.Request(pkt)
			sendToLLDB(devReply)
			return

		default:
			// forward to device (devGDB uses ack mode: sends "+$pkt#XX")
			devReply, err := devGDB.Request(pkt)
			if err != nil {
				utils.Verbose("lldb-proxy: device error for %q: %v", pkt[:min(len(pkt), 40)], err)
				return
			}
			reply = devReply
		}

		sendToLLDB(reply)
		if switchToNoAck {
			noAck = true
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// injectServerViaLLDB connects LLDB to the proxy (which has already attached to
// the target process), evaluates iosDeviceAgentExpr to start a persistent HTTP
// server inside the app, and returns the device-side TCP port.
func injectServerViaLLDB(localProxyPort int) (int, error) {
	const lldbTimeout = 120 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), lldbTimeout)
	defer cancel()

	utils.Verbose("running LLDB (timeout %s)", lldbTimeout)
	cmd := exec.CommandContext(ctx, "lldb",
		"-o", "platform select remote-ios",
		"-o", fmt.Sprintf("process connect connect://localhost:%d", localProxyPort),
		"-o", "expr -l objc -- "+iosDeviceAgentExpr,
		"-o", "detach",
		"-o", "quit",
	)
	out, err := cmd.CombinedOutput()
	utils.Verbose("LLDB finished (err=%v), output:\n%s", err, out)
	if err != nil {
		return 0, fmt.Errorf("lldb: %w\noutput:\n%s", err, out)
	}

	for _, line := range strings.Split(string(out), "\n") {
		m := portFromLLDB.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		port, err := strconv.Atoi(m[1])
		if err == nil && port >= 27042 && port <= 27051 {
			return port, nil
		}
	}
	return 0, fmt.Errorf("could not parse port from lldb output:\n%s", out)
}

// findFreeLocalPort returns the first available local TCP port in the given range.
func findFreeLocalPort(start, end int) (int, error) {
	for p := start; p <= end; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			ln.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in range %d-%d", start, end)
}

func (d *IOSDevice) ensureIOSDeviceAgentReady() (int, error) {
	// fast path: reuse the forwarded port we set up for this device previously
	if port, ok := cachedAgentPort(d.Udid); ok && isAgentReady(port) {
		utils.Verbose("reusing cached agent port %d", port)
		return port, nil
	}

	if err := d.startTunnel(); err != nil {
		return 0, fmt.Errorf("start tunnel: %w", err)
	}
	utils.Verbose("getting enhanced device info")
	device, err := d.getEnhancedDevice()
	if err != nil {
		return 0, fmt.Errorf("get enhanced device: %w", err)
	}
	proxyPort, err := iosDeviceDebugProxyPort(device)
	if err != nil {
		return 0, err
	}
	utils.Verbose("debug proxy port from RSD: %d", proxyPort)

	utils.Verbose("listing running user apps")
	apps, err := d.userApps(device)
	if err != nil {
		return 0, err
	}
	if len(apps) == 0 {
		return 0, fmt.Errorf("no user app running — open an app first")
	}

	utils.Verbose("finding foreground app among %d candidates", len(apps))
	foreground, err := d.findForegroundApp(device, apps)
	if err != nil {
		return 0, err
	}

	utils.Verbose("injecting agent into %s (pid %d) via LLDB", foreground.bundleID, foreground.pid)
	// start a local TCP proxy so LLDB can reach the device debug proxy
	lldbProxyPort, cancelProxy, err := startLLDBProxy(device, proxyPort, foreground.pid)
	if err != nil {
		return 0, fmt.Errorf("start lldb proxy: %w", err)
	}
	defer cancelProxy()
	utils.Verbose("LLDB proxy listening on localhost:%d", lldbProxyPort)

	devicePort, err := injectServerViaLLDB(lldbProxyPort)
	if err != nil {
		return 0, fmt.Errorf("inject server via lldb: %w", err)
	}
	utils.Verbose("agent started on device port %d", devicePort)

	localPort, err := findFreeLocalPort(27042, 27051)
	if err != nil {
		return 0, err
	}

	utils.Verbose("forwarding localhost:%d -> device:%d", localPort, devicePort)
	pf := iosutil.NewPortForwarder(d.Udid)
	if err := pf.Forward(localPort, devicePort); err != nil {
		return 0, fmt.Errorf("port forward %d->%d: %w", localPort, devicePort, err)
	}

	utils.Verbose("waiting for agent to respond on port %d", localPort)
	deadline := time.Now().Add(3 * time.Second)
	for !isAgentReady(localPort) {
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("iOS device agent did not respond within 3s")
		}
		time.Sleep(100 * time.Millisecond)
	}
	utils.Verbose("agent ready on port %d", localPort)
	setCachedAgentPort(d.Udid, localPort)
	return localPort, nil
}


