@import Foundation;
// UIKit/WebKit class metadata avoided — calls cast result to silence LLDB strict mode
extern Class objc_getClass(const char *);
// Geometry helpers: UIKit headers aren't on the include path, so to call
// struct-returning methods (-bounds, -convertRect:toView:) we declare a
// CGRect-compatible struct and call through cast'd objc_msgSend (CGFloat is
// double on arm64; arm64 has no objc_msgSend_stret, so the plain symbol works).
extern void objc_msgSend(void);
extern SEL sel_registerName(const char *);
typedef double __mcfloat;
typedef struct { __mcfloat x, y; } __mcpoint;
typedef struct { __mcfloat width, height; } __mcsize;
typedef struct { __mcpoint origin; __mcsize size; } __mcrect;
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
extern int *__error(void); // Darwin errno is (*__error())
extern __in_port_t htons(__in_port_t);
extern __in_addr_t htonl(__in_addr_t);
extern void *memset(void *, int, unsigned long);
extern char *strdup(const char *);
extern void free(void *);
extern unsigned long strlen(const char *);
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
__sa.sin_port = htons((unsigned short)12008);
if (bind(__sfd, (struct __sockaddr *)&__sa, sizeof(__sa)) == 0) { __port = 12008; }
if (__port > 0) {
    listen(__sfd, 8);
    int __srv = __sfd;
    dispatch_async(dispatch_get_global_queue(0, 0), ^{
        id (^__findWV)(NSString *) = ^id(NSString *wvId) {
            __block id found = nil;
            id sem = dispatch_semaphore_create(0);
            [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                Class wk = NSClassFromString(@"WKWebView");
                Class wsCls = (Class)objc_getClass("UIWindowScene");
                id app = (id)[(Class)objc_getClass("UIApplication") sharedApplication];
                for (id sc in (NSArray *)[app connectedScenes])
                    if ([(NSObject *)sc isKindOfClass:wsCls])
                        for (id w in (NSArray *)[sc windows]) {
                            NSMutableArray *stk = [NSMutableArray arrayWithObject:w];
                            while ([stk count]) {
                                id v = stk[0]; [stk removeObjectAtIndex:0];
                                if (wk && [(NSObject *)v isKindOfClass:wk] &&
                                    [[NSString stringWithFormat:@"%p", v] isEqualToString:wvId])
                                { found = v; break; }
                                [stk addObjectsFromArray:(NSArray *)[v subviews]];
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
                        Class wsCls = (Class)objc_getClass("UIWindowScene");
                        id app = (id)[(Class)objc_getClass("UIApplication") sharedApplication];
                        for (id sc in (NSArray *)[app connectedScenes])
                            if ([(NSObject *)sc isKindOfClass:wsCls])
                                for (id win in (NSArray *)[sc windows]) {
                                    NSMutableArray *stk = [NSMutableArray arrayWithObject:win];
                                    while ([stk count]) {
                                        id v = stk[0]; [stk removeObjectAtIndex:0];
                                        if (wk && [(NSObject *)v isKindOfClass:wk]) {
                                            __mcrect (*__boundsMsg)(id, SEL) = (__mcrect (*)(id, SEL))objc_msgSend;
                                            __mcrect (*__convMsg)(id, SEL, __mcrect, id) = (__mcrect (*)(id, SEL, __mcrect, id))objc_msgSend;
                                            __mcrect __fr = __convMsg(v, sel_registerName("convertRect:toView:"), __boundsMsg(v, sel_registerName("bounds")), (id)0);
                                            [wvs addObject:@{
                                                @"id": [NSString stringWithFormat:@"%p", v],
                                                @"url": [(NSURL *)[v URL] absoluteString] ?: @"",
                                                @"title": (NSString *)[v title] ?: @"",
                                                @"bounds": @{@"x":@(__fr.origin.x),@"y":@(__fr.origin.y),@"width":@(__fr.size.width),@"height":@(__fr.size.height)},
                                                @"visible": @(!(BOOL)[v isHidden] && (id)[v window] != nil)
                                            }];
                                        }
                                        [stk addObjectsFromArray:(NSArray *)[v subviews]];
                                    }
                                }
                        dispatch_semaphore_signal(sem);
                    }];
                    dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                    resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":wvs} options:0 error:nil];
                } else if ([method isEqualToString:@"device.webview.goto"]) {
                    NSString *wvId = params[@"id"], *url = params[@"url"];
                    id wv = (wvId && url) ? __findWV(wvId) : nil;
                    if (!wvId || !url) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id or url"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{ (void)[wv loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:url]]]; dispatch_semaphore_signal(sem); }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                        resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"status":@"ok"}} options:0 error:nil];
                    }
                } else if ([@[@"device.webview.reload",@"device.webview.goBack",@"device.webview.goForward"] containsObject:method]) {
                    NSString *wvId = params[@"id"];
                    id wv = wvId ? __findWV(wvId) : nil;
                    if (!wvId) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                            if ([method isEqualToString:@"device.webview.reload"]) (void)[wv reload];
                            else if ([method isEqualToString:@"device.webview.goBack"]) (void)[wv goBack];
                            else (void)[wv goForward];
                            dispatch_semaphore_signal(sem);
                        }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 5000000000LL));
                        resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":@{@"status":@"ok"}} options:0 error:nil];
                    }
                } else if ([method isEqualToString:@"device.webview.evaluate"]) {
                    NSString *wvId = params[@"id"], *expr = params[@"expression"];
                    id wv = (wvId && expr) ? __findWV(wvId) : nil;
                    if (!wvId || !expr) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id or expression"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        // A heap (non-tagged) value returned from evaluateJavaScript is
                        // over-released when WebKit's delivery pool drains, crashing the
                        // app — so we must never hold WebKit's result object. Instead the
                        // JS JSON-stringifies [ok, value]; inside the handler we copy the
                        // resulting bytes to C memory (no ObjC retain) and rebuild the
                        // response from our own bytes. Arrays + a mutable dict avoid the
                        // single-entry immutable dictionaries seen in the crash.
                        NSString *wrapped = [NSString stringWithFormat:@"(function(){try{return JSON.stringify([1,(function(){%@})()])}catch(e){return JSON.stringify([0,''+e])}})()", expr];
                        __block char *jbuf = NULL;
                        id sem = dispatch_semaphore_create(0);
                        [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                            (void)[wv evaluateJavaScript:wrapped completionHandler:^(id r, NSError *e) {
                                if ([(NSObject *)r isKindOfClass:[NSString class]]) { const char *u = [(NSString *)r UTF8String]; if (u) jbuf = strdup(u); }
                                dispatch_semaphore_signal(sem);
                            }];
                        }];
                        dispatch_semaphore_wait(sem, dispatch_time(0, 30000000000LL));
                        if (!jbuf) {
                            resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"no result from evaluate"}} options:0 error:nil];
                        } else {
                            NSArray *parsed = [NSJSONSerialization JSONObjectWithData:[NSData dataWithBytes:jbuf length:strlen(jbuf)] options:0 error:nil];
                            free(jbuf);
                            BOOL ok2 = [(NSObject *)parsed isKindOfClass:[NSArray class]] && [parsed count] == 2;
                            if (ok2 && [(NSNumber *)parsed[0] intValue] == 0) {
                                resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":parsed[1]}} options:0 error:nil];
                            } else {
                                NSMutableDictionary *rd = [NSMutableDictionary dictionary];
                                rd[@"result"] = ok2 ? parsed[1] : [NSNull null];
                                resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"result":rd} options:0 error:nil];
                            }
                        }
                    }
                } else if ([method isEqualToString:@"device.webview.waitForLoadState"]) {
                    NSString *wvId = params[@"id"];
                    id wv = wvId ? __findWV(wvId) : nil;
                    if (!wvId) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32602),@"message":@"missing id"}} options:0 error:nil];
                    else if (!wv) resp = [NSJSONSerialization dataWithJSONObject:@{@"jsonrpc":@"2.0",@"id":rqId,@"error":@{@"code":@(-32000),@"message":@"webview not found"}} options:0 error:nil];
                    else {
                        NSString *__lstate = params[@"state"] ?: @"load";
                        NSInteger toMs = params[@"timeout"] ? [(NSNumber *)params[@"timeout"] integerValue] : 30000;
                        NSString *checkJS = [@"domcontentloaded" isEqualToString:__lstate] ?
                            @"return String(document.readyState==='interactive'||document.readyState==='complete')" :
                            @"return String(document.readyState==='complete')";
                        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:toMs / 1000.0];
                        BOOL done = NO;
                        while (!done && [[NSDate date] compare:deadline] == NSOrderedAscending) {
                            __block id jsR = nil;
                            id sem2 = dispatch_semaphore_create(0);
                            NSString *wrapped = [NSString stringWithFormat:@"(function(){try{%@}catch(e){return null}})()", checkJS];
                            [[NSOperationQueue mainQueue] addOperationWithBlock:^{
                                (void)[wv evaluateJavaScript:wrapped completionHandler:^(id r, NSError *e) { jsR = r; dispatch_semaphore_signal(sem2); }];
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
                // send() can short-write (notably on EINTR); loop until the full
                // header + body is written, advancing via unsigned char* arithmetic.
                BOOL (^__sendAll)(const void *, unsigned long) = ^BOOL(const void *b, unsigned long n) {
                    const unsigned char *p = (const unsigned char *)b;
                    unsigned long left = n;
                    while (left > 0) {
                        long w = send(cfd, p, left, 0);
                        if (w > 0) { p += w; left -= (unsigned long)w; }
                        else if (w < 0 && (*__error()) == 4 /* EINTR */) { continue; }
                        else { return NO; }
                    }
                    return YES;
                };
                if (__sendAll(hdrData.bytes, hdrData.length)) __sendAll(resp.bytes, resp.length);
                close(cfd);
            });
        }
    });
}
(int)__port
