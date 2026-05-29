#import "dispatcher.h"
#import "bridge.h"

static const int kRPCParseError     = -32700;
static const int kRPCMethodNotFound = -32601;
static const int kRPCInvalidParams  = -32602;
static const int kRPCServerError    = -32000;

static NSData *rpc_result(id reqId, id value) {
    NSDictionary *resp = @{@"jsonrpc": @"2.0", @"id": reqId ?: [NSNull null], @"result": value};
    return [NSJSONSerialization dataWithJSONObject:resp options:0 error:nil];
}

static NSData *rpc_error(id reqId, int code, NSString *message) {
    NSDictionary *resp = @{
        @"jsonrpc": @"2.0",
        @"id":      reqId ?: [NSNull null],
        @"error":   @{@"code": @(code), @"message": message},
    };
    return [NSJSONSerialization dataWithJSONObject:resp options:0 error:nil];
}

static NSString *requireParam(id reqId, NSDictionary *params, NSString *key, NSData * _Nullable * _Nonnull outError) {
    NSString *v = params[key];
    if (!v || [v isEqual:[NSNull null]] || ((NSString *)v).length == 0) {
        *outError = rpc_error(reqId, kRPCInvalidParams, [NSString stringWithFormat:@"missing params.%@", key]);
        return nil;
    }
    return v;
}

NSData *dispatch_rpc(NSData *body) {
    NSError *jsonErr = nil;
    NSDictionary *req = [NSJSONSerialization JSONObjectWithData:body options:0 error:&jsonErr];
    if (!req || jsonErr) {
        return rpc_error([NSNull null], kRPCParseError, @"parse error");
    }

    id reqId         = req[@"id"] ?: [NSNull null];
    NSString *method = req[@"method"];
    NSDictionary *params = req[@"params"];

    if ([method isEqualToString:@"device.webview.list"]) {
        return rpc_result(reqId, [IosBridge listWebViews]);
    }

    if ([method isEqualToString:@"device.webview.goto"]) {
        NSData *err = nil;
        NSString *wvId = requireParam(reqId, params, @"id", &err);  if (!wvId) return err;
        NSString *url  = requireParam(reqId, params, @"url", &err); if (!url)  return err;
        UIView *wv = [IosBridge webViewWithID:wvId];
        if (!wv) return rpc_error(reqId, kRPCServerError, [NSString stringWithFormat:@"webview not found: %@", wvId]);
        [IosBridge gotoURL:url inWebView:wv];
        return rpc_result(reqId, @{@"status": @"ok"});
    }

    if ([method isEqualToString:@"device.webview.evaluate"]) {
        NSData *err = nil;
        NSString *wvId       = requireParam(reqId, params, @"id", &err);         if (!wvId)       return err;
        NSString *expression = requireParam(reqId, params, @"expression", &err); if (!expression) return err;
        UIView *wv = [IosBridge webViewWithID:wvId];
        if (!wv) return rpc_error(reqId, kRPCServerError, [NSString stringWithFormat:@"webview not found: %@", wvId]);
        NSDictionary *eval = [IosBridge evaluateJS:expression inWebView:wv];
        if (eval[@"__error"]) return rpc_error(reqId, kRPCServerError, eval[@"__error"]);
        return rpc_result(reqId, eval);
    }

    if ([@[@"device.webview.reload", @"device.webview.goBack", @"device.webview.goForward"] containsObject:method]) {
        NSData *err = nil;
        NSString *wvId = requireParam(reqId, params, @"id", &err);
        if (!wvId) return err;
        UIView *wv = [IosBridge webViewWithID:wvId];
        if (!wv) return rpc_error(reqId, kRPCServerError, [NSString stringWithFormat:@"webview not found: %@", wvId]);
        if ([method isEqualToString:@"device.webview.reload"])        [IosBridge reloadWebView:wv];
        else if ([method isEqualToString:@"device.webview.goBack"])    [IosBridge goBackWebView:wv];
        else if ([method isEqualToString:@"device.webview.goForward"]) [IosBridge goForwardWebView:wv];
        return rpc_result(reqId, @{@"status": @"ok"});
    }

    if ([method isEqualToString:@"device.webview.waitForLoadState"]) {
        NSData *err = nil;
        NSString *wvId = requireParam(reqId, params, @"id", &err);
        if (!wvId) return err;
        UIView *wv = [IosBridge webViewWithID:wvId];
        if (!wv) return rpc_error(reqId, kRPCServerError, [NSString stringWithFormat:@"webview not found: %@", wvId]);

        NSString *state     = params[@"state"] ?: @"load";
        NSInteger timeoutMs = params[@"timeout"] ? [params[@"timeout"] integerValue] : 30000;

        NSString *js = [@"domcontentloaded" isEqualToString:state]
            ? @"return String(document.readyState === 'interactive' || document.readyState === 'complete')"
            : @"return String(document.readyState === 'complete')";

        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:timeoutMs / 1000.0];
        while (YES) {
            NSDictionary *result = [IosBridge evaluateJS:js inWebView:wv];
            if ([@"true" isEqualToString:result[@"result"]]) {
                return rpc_result(reqId, @{@"status": @"ok"});
            }
            if ([[NSDate date] compare:deadline] != NSOrderedAscending) {
                return rpc_error(reqId, kRPCServerError,
                    [NSString stringWithFormat:@"waitForLoadState timed out waiting for '%@'", state]);
            }
            [NSThread sleepForTimeInterval:0.2];
        }
    }

    static NSArray *stubMethods;
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        stubMethods = @[@"device.dump.ui"];
    });
    if ([stubMethods containsObject:method]) {
        return rpc_error(reqId, kRPCServerError, [NSString stringWithFormat:@"not yet implemented: %@", method]);
    }

    return rpc_error(reqId, kRPCMethodNotFound, [NSString stringWithFormat:@"method not found: %@", method]);
}
