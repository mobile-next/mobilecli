#import <Foundation/Foundation.h>
#import <objc/runtime.h>

typedef NSURLSessionDataTask *(*DataTaskIMP)(id, SEL, NSURL *, void (^)(NSData *, NSURLResponse *, NSError *));

static DataTaskIMP originalDataTaskWithURL = NULL;

static NSURLSessionDataTask *swizzledDataTaskWithURL(id self, SEL _cmd, NSURL *url,
                                                     void (^handler)(NSData *, NSURLResponse *, NSError *)) {
    void (^wrapped)(NSData *, NSURLResponse *, NSError *) = ^(NSData *data, NSURLResponse *resp, NSError *err) {
        if (data != nil) {
            NSString *body = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding]
                             ?: @"<non-utf8 body>";
            NSString *statusStr = [resp isKindOfClass:[NSHTTPURLResponse class]]
                ? [NSString stringWithFormat:@"%ld", (long)((NSHTTPURLResponse *)resp).statusCode]
                : @"non-HTTP response";
            NSString *out = [NSString stringWithFormat:@"url: %@\nstatus: %@\nbody:\n%@\n",
                             url.absoluteString, statusStr, body];
            [out writeToFile:@"/tmp/gilm.txt"
                  atomically:YES
                    encoding:NSUTF8StringEncoding
                       error:nil];
            NSLog(@"[gadget] swizzle fired for %@ -> %ld, wrote to /tmp/gilm.txt", url, (long)http.statusCode);
        }
        if (handler) handler(data, resp, err);
    };
    return originalDataTaskWithURL(self, _cmd, url, wrapped);
}

__attribute__((constructor))
static void on_load(void) {
    // write a marker so we know the dylib loaded even before the request fires
    [@"gadget loaded\n" writeToFile:@"/tmp/gilm.txt"
                         atomically:YES
                           encoding:NSUTF8StringEncoding
                              error:nil];

    Class cls = [NSURLSession class];
    SEL sel = @selector(dataTaskWithURL:completionHandler:);
    Method m = class_getInstanceMethod(cls, sel);
    originalDataTaskWithURL = (DataTaskIMP)method_setImplementation(m, (IMP)swizzledDataTaskWithURL);

    NSLog(@"[gadget] swizzled NSURLSession -dataTaskWithURL:completionHandler:");
}
