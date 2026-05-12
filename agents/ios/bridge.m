#import "bridge.h"
#import <UIKit/UIKit.h>
#import <WebKit/WebKit.h>

@implementation IosBridge

+ (void)runOnMainThread:(dispatch_block_t)block {
    if ([NSThread isMainThread]) {
        block();
        return;
    }
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    dispatch_async(dispatch_get_main_queue(), ^{
        block();
        dispatch_semaphore_signal(sem);
    });
    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
}

+ (NSArray<UIWindow *> *)allWindows {
    NSMutableArray<UIWindow *> *windows = [NSMutableArray array];
    for (UIScene *scene in [UIApplication sharedApplication].connectedScenes) {
        if ([scene isKindOfClass:[UIWindowScene class]]) {
            [windows addObjectsFromArray:((UIWindowScene *)scene).windows];
        }
    }
    return windows;
}

+ (void)collectWebViews:(UIView *)view into:(NSMutableArray<NSDictionary *> *)result depth:(int)depth {
    if (depth > 50) return;

    Class wkClass = NSClassFromString(@"WKWebView");
    if (wkClass && [view isKindOfClass:wkClass]) {
        NSURL *url      = [view valueForKey:@"URL"];
        NSString *title = [view valueForKey:@"title"];
        CGRect frame    = [view convertRect:view.bounds toView:nil];
        BOOL visible    = !view.isHidden && view.alpha > 0.01 && view.window != nil;

        [result addObject:@{
            @"id":      [NSString stringWithFormat:@"%p", view],
            @"url":     url.absoluteString ?: @"",
            @"title":   title ?: @"",
            @"bounds":  @{
                @"x":      @(frame.origin.x),
                @"y":      @(frame.origin.y),
                @"width":  @(frame.size.width),
                @"height": @(frame.size.height),
            },
            @"visible": @(visible),
        }];
    }

    for (UIView *child in view.subviews) {
        [self collectWebViews:child into:result depth:depth + 1];
    }
}

+ (NSArray<NSDictionary *> *)listWebViews {
    __block NSMutableArray<NSDictionary *> *result = [NSMutableArray array];
    [self runOnMainThread:^{
        for (UIWindow *window in [self allWindows]) {
            [self collectWebViews:window into:result depth:0];
        }
    }];
    return result;
}

+ (UIView *)findView:(UIView *)view withID:(NSString *)wvId wkClass:(Class)wkClass depth:(int)depth {
    if (depth > 50) return nil;
    if ([view isKindOfClass:wkClass] && [[NSString stringWithFormat:@"%p", view] isEqualToString:wvId]) {
        return view;
    }
    for (UIView *child in view.subviews) {
        UIView *found = [self findView:child withID:wvId wkClass:wkClass depth:depth + 1];
        if (found) return found;
    }
    return nil;
}

+ (UIView *)webViewWithID:(NSString *)wvId {
    Class wkClass = NSClassFromString(@"WKWebView");
    if (!wkClass) return nil;
    __block UIView *found = nil;
    [self runOnMainThread:^{
        for (UIWindow *window in [self allWindows]) {
            found = [self findView:window withID:wvId wkClass:wkClass depth:0];
            if (found) break;
        }
    }];
    return found;
}

// returns {"result": <value>} on success, {"__error": <message>} on failure
+ (NSDictionary *)evaluateJS:(NSString *)expression inWebView:(UIView *)webView {
    NSString *wrapped = [NSString stringWithFormat:
        @"(function(){try{%@}catch(e){return {__mce:e.toString()}}})()",
        expression];

    __block id jsResult = nil;
    __block NSError *jsError = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);

    dispatch_async(dispatch_get_main_queue(), ^{
        [(WKWebView *)webView evaluateJavaScript:wrapped completionHandler:^(id result, NSError *error) {
            jsResult = result;
            jsError = error;
            dispatch_semaphore_signal(sem);
        }];
    });
    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));

    if (jsError) {
        return @{@"__error": jsError.localizedDescription};
    }
    if ([jsResult isKindOfClass:[NSDictionary class]] && jsResult[@"__mce"]) {
        return @{@"__error": jsResult[@"__mce"] ?: @"unknown JS error"};
    }
    return @{@"result": jsResult ?: [NSNull null]};
}

+ (void)gotoURL:(NSString *)urlStr inWebView:(UIView *)webView {
    [self runOnMainThread:^{
        NSURL *url = [NSURL URLWithString:urlStr];
        [(WKWebView *)webView loadRequest:[NSURLRequest requestWithURL:url]];
    }];
}

@end
