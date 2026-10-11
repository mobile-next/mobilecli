#import "network.h"
#import "network_glob.h"
#import <objc/runtime.h>

static const NSTimeInterval kDecisionTimeout = 60;
static const NSTimeInterval kPollTimeout = 30;
static const NSTimeInterval kPollGrace = 15;
static const NSUInteger kMaxEvents = 256;
static const NSUInteger kMaxEventBody = 256 * 1024;
static NSString *const kHandledKey = @"mobilecli.network.handled";
static NSString *const kErrorDomain = @"mobilecli.network";

@interface MobilePause : NSObject
@property (nonatomic, strong) NSDictionary *event;
@property (nonatomic, strong) NSDictionary *verdict;
@end
@implementation MobilePause
@end

@interface MobileURLProtocol : NSURLProtocol
@property (atomic, assign) BOOL stopped;
@property (atomic, strong) NSURLSession *forwardSession;
@property (atomic, strong) NSURLSessionDataTask *forwardTask;
@end

@interface MobileNetwork (Internals)
+ (BOOL)matches:(NSURLRequest *)request;
+ (nullable NSString *)pauseRequest:(NSURLRequest *)request;
+ (NSDictionary *)waitFor:(NSString *)requestId;
+ (void)cancel:(nullable NSString *)requestId;
@end

static NSCondition *stateLock;
static NSMutableArray<NSDictionary *> *routes;
static NSMutableArray<NSDictionary *> *events;
static NSMutableDictionary<NSString *, MobilePause *> *paused;
static BOOL enabled;
static NSUInteger sequence;
static NSInteger pollers;
static NSDate *lastPollEnd;
static NSURLSession *captureSession;
static const void *kDelegateCloneKey = &kDelegateCloneKey;
static const void *kIsDelegateCloneKey = &kIsDelegateCloneKey;

static NSError *invalid(NSString *message) {
    return [NSError errorWithDomain:kErrorDomain code:-32602
                          userInfo:@{NSLocalizedDescriptionKey: message}];
}

static BOOL matchesLocked(NSURLRequest *request) {
    if (!enabled || !request.URL) {
        return NO;
    }
    NSString *url = request.URL.absoluteString;
    NSString *method = request.HTTPMethod ?: @"GET";
    for (NSDictionary *route in routes) {
        NSString *wanted = route[@"method"];
        if (wanted && [wanted caseInsensitiveCompare:method] != NSOrderedSame) {
            continue;
        }
        NSRegularExpression *pattern = route[@"pattern"];
        if ([pattern numberOfMatchesInString:url options:0 range:NSMakeRange(0, url.length)] > 0) {
            return YES;
        }
    }
    return NO;
}

static void releaseAllLocked(void) {
    for (MobilePause *pause in paused.allValues) {
        pause.verdict = @{@"action": @"continue"};
    }
    [paused removeAllObjects];
    [stateLock broadcast];
}

@interface NSURLSessionConfiguration (MobileNetworkConfiguration)
+ (NSURLSessionConfiguration *)mobile_defaultSessionConfiguration;
+ (NSURLSessionConfiguration *)mobile_ephemeralSessionConfiguration;
@end

static void addProtocol(NSURLSessionConfiguration *configuration) {
    NSMutableArray<Class> *classes = [configuration.protocolClasses mutableCopy] ?: [NSMutableArray array];
    if (![classes containsObject:[MobileURLProtocol class]]) {
        [classes insertObject:[MobileURLProtocol class] atIndex:0];
        configuration.protocolClasses = classes;
    }
}

@implementation NSURLSessionConfiguration (MobileNetworkConfiguration)
+ (NSURLSessionConfiguration *)mobile_defaultSessionConfiguration {
    NSURLSessionConfiguration *configuration = [self mobile_defaultSessionConfiguration];
    addProtocol(configuration);
    return configuration;
}
+ (NSURLSessionConfiguration *)mobile_ephemeralSessionConfiguration {
    NSURLSessionConfiguration *configuration = [self mobile_ephemeralSessionConfiguration];
    addProtocol(configuration);
    return configuration;
}
@end

@interface NSURLSession (MobileNetworkTasks)
- (NSURLSessionDataTask *)mobile_dataTaskWithURL:(NSURL *)url
    completionHandler:(void (^)(NSData *, NSURLResponse *, NSError *))completionHandler;
- (NSURLSessionDataTask *)mobile_dataTaskWithRequest:(NSURLRequest *)request
    completionHandler:(void (^)(NSData *, NSURLResponse *, NSError *))completionHandler;
- (NSURLSessionDataTask *)mobile_dataTaskWithRequest:(NSURLRequest *)request;
@end

@implementation NSURLSession (MobileNetworkTasks)
- (NSURLSessionDataTask *)mobile_dataTaskWithURL:(NSURL *)url
    completionHandler:(void (^)(NSData *, NSURLResponse *, NSError *))completionHandler {
    if (self != captureSession && [MobileNetwork matches:[NSURLRequest requestWithURL:url]]) {
        return [captureSession dataTaskWithURL:url completionHandler:completionHandler];
    }
    return [self mobile_dataTaskWithURL:url completionHandler:completionHandler];
}

- (NSURLSessionDataTask *)mobile_dataTaskWithRequest:(NSURLRequest *)request
    completionHandler:(void (^)(NSData *, NSURLResponse *, NSError *))completionHandler {
    if (self != captureSession && [MobileNetwork matches:request]) {
        return [captureSession dataTaskWithRequest:request completionHandler:completionHandler];
    }
    return [self mobile_dataTaskWithRequest:request completionHandler:completionHandler];
}

- (NSURLSessionDataTask *)mobile_dataTaskWithRequest:(NSURLRequest *)request {
    if (![objc_getAssociatedObject(self, kIsDelegateCloneKey) boolValue] &&
        [MobileNetwork matches:request]) {
        NSURLSession *clone = objc_getAssociatedObject(self, kDelegateCloneKey);
        if (!clone) {
            @synchronized (self) {
                clone = objc_getAssociatedObject(self, kDelegateCloneKey);
                if (!clone) {
                    NSURLSessionConfiguration *configuration = [self.configuration copy];
                    addProtocol(configuration);
                    clone = [NSURLSession sessionWithConfiguration:configuration
                        delegate:self.delegate delegateQueue:self.delegateQueue];
                    objc_setAssociatedObject(clone, kIsDelegateCloneKey, @YES, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
                    objc_setAssociatedObject(self, kDelegateCloneKey, clone, OBJC_ASSOCIATION_RETAIN_NONATOMIC);
                }
            }
        }
        return [clone dataTaskWithRequest:request];
    }
    return [self mobile_dataTaskWithRequest:request];
}
@end

@implementation MobileNetwork
+ (void)initialize {
    if (self != [MobileNetwork class]) {
        return;
    }
    stateLock = [NSCondition new];
    routes = [NSMutableArray array];
    events = [NSMutableArray array];
    paused = [NSMutableDictionary dictionary];
    lastPollEnd = [NSDate date];
}

+ (void)install {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        [NSURLProtocol registerClass:[MobileURLProtocol class]];
        Class meta = object_getClass([NSURLSessionConfiguration class]);
        method_exchangeImplementations(
            class_getInstanceMethod(meta, @selector(defaultSessionConfiguration)),
            class_getInstanceMethod(meta, @selector(mobile_defaultSessionConfiguration)));
        method_exchangeImplementations(
            class_getInstanceMethod(meta, @selector(ephemeralSessionConfiguration)),
            class_getInstanceMethod(meta, @selector(mobile_ephemeralSessionConfiguration)));
        NSURLSessionConfiguration *configuration = [NSURLSessionConfiguration ephemeralSessionConfiguration];
        addProtocol(configuration);
        captureSession = [NSURLSession sessionWithConfiguration:configuration];
        method_exchangeImplementations(
            class_getInstanceMethod([NSURLSession class], @selector(dataTaskWithURL:completionHandler:)),
            class_getInstanceMethod([NSURLSession class], @selector(mobile_dataTaskWithURL:completionHandler:)));
        method_exchangeImplementations(
            class_getInstanceMethod([NSURLSession class], @selector(dataTaskWithRequest:completionHandler:)),
            class_getInstanceMethod([NSURLSession class], @selector(mobile_dataTaskWithRequest:completionHandler:)));
        method_exchangeImplementations(
            class_getInstanceMethod([NSURLSession class], @selector(dataTaskWithRequest:)),
            class_getInstanceMethod([NSURLSession class], @selector(mobile_dataTaskWithRequest:)));
        NSLog(@"[mobilecli] URLSession network hook installed");
    });
}

+ (BOOL)matches:(NSURLRequest *)request {
    [stateLock lock];
    BOOL match = matchesLocked(request);
    [stateLock unlock];
    return match;
}

+ (nullable NSString *)pauseRequest:(NSURLRequest *)request {
    [stateLock lock];
    if (!matchesLocked(request) || events.count >= kMaxEvents) {
        [stateLock unlock];
        return nil;
    }
    NSString *requestId = [NSString stringWithFormat:@"%lu", (unsigned long)++sequence];
    NSData *body = request.HTTPBody;
    NSMutableDictionary *event = [@{
        @"type": @"paused", @"requestId": requestId,
        @"method": request.HTTPMethod ?: @"GET",
        @"url": request.URL.absoluteString ?: @"",
        @"headers": request.allHTTPHeaderFields ?: @{},
        @"body": body ? [[body subdataWithRange:NSMakeRange(0, MIN(body.length, kMaxEventBody))]
                         base64EncodedStringWithOptions:0] : [NSNull null],
        @"stack": @"urlsession",
    } mutableCopy];
    if (body.length > kMaxEventBody) {
        event[@"bodyTruncated"] = @YES;
    }
    MobilePause *pause = [MobilePause new];
    pause.event = event;
    paused[requestId] = pause;
    [events addObject:event];
    [stateLock broadcast];
    [stateLock unlock];
    return requestId;
}

+ (NSDictionary *)waitFor:(NSString *)requestId {
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:kDecisionTimeout];
    [stateLock lock];
    MobilePause *pause = paused[requestId];
    while (pause && !pause.verdict) {
        NSDate *now = [NSDate date];
        if (!enabled || [now compare:deadline] != NSOrderedAscending ||
            (pollers == 0 && [now timeIntervalSinceDate:lastPollEnd] >= kPollGrace)) {
            pause.verdict = @{@"action": @"continue"};
            break;
        }
        [stateLock waitUntilDate:[NSDate dateWithTimeIntervalSinceNow:1]];
    }
    NSDictionary *verdict = pause.verdict ?: @{@"action": @"continue"};
    [paused removeObjectForKey:requestId];
    if (pause.event) {
        [events removeObject:pause.event];
    }
    [stateLock unlock];
    return verdict;
}

+ (void)cancel:(NSString *)requestId {
    if (!requestId) {
        return;
    }
    [stateLock lock];
    MobilePause *pause = paused[requestId];
    if (pause) {
        pause.verdict = @{@"action": @"continue"};
        [paused removeObjectForKey:requestId];
        [events removeObject:pause.event];
        [stateLock broadcast];
    }
    [stateLock unlock];
}

+ (nullable NSDictionary *)perform:(NSString *)method params:(NSDictionary *)params error:(NSError **)error {
    if ([method isEqualToString:@"network.enable"]) {
        id definitions = params[@"routes"];
        if (![definitions isKindOfClass:[NSArray class]]) {
            if (error) {
                *error = invalid(@"routes must be an array");
            }
            return nil;
        }
        NSMutableArray *next = [NSMutableArray array];
        for (id definition in definitions) {
            if (![definition isKindOfClass:[NSDictionary class]] ||
                ![definition[@"url"] isKindOfClass:[NSString class]] ||
                [definition[@"url"] length] == 0) {
                if (error) {
                    *error = invalid(@"route.url must be a nonempty string");
                }
                return nil;
            }
            NSRegularExpression *pattern = MobileCompileNetworkGlob(definition[@"url"]);
            if (!pattern) {
                if (error) {
                    *error = invalid(@"invalid route glob");
                }
                return nil;
            }
            NSMutableDictionary *route = [@{@"pattern": pattern} mutableCopy];
            if ([definition[@"method"] isKindOfClass:[NSString class]] && [definition[@"method"] length]) {
                route[@"method"] = definition[@"method"];
            }
            [next addObject:route];
        }
        if (!next.count) {
            [next addObject:@{@"pattern": MobileCompileNetworkGlob(@"**")}];
        }
        [stateLock lock];
        releaseAllLocked();
        [events removeAllObjects];
        routes = next;
        enabled = YES;
        lastPollEnd = [NSDate date];
        [stateLock broadcast];
        [stateLock unlock];
        return @{@"status": @"ok"};
    }
    if ([method isEqualToString:@"network.poll"]) {
        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:kPollTimeout];
        [stateLock lock];
        pollers++;
        while (enabled && !events.count && [[NSDate date] compare:deadline] == NSOrderedAscending) {
            [stateLock waitUntilDate:deadline];
        }
        NSArray *batch = [events copy];
        [events removeAllObjects];
        pollers--;
        lastPollEnd = [NSDate date];
        [stateLock unlock];
        return @{@"events": batch};
    }
    if ([method isEqualToString:@"network.disable"]) {
        [stateLock lock];
        enabled = NO;
        [routes removeAllObjects];
        [events removeAllObjects];
        releaseAllLocked();
        [stateLock unlock];
        return @{@"status": @"ok"};
    }
    if ([method isEqualToString:@"network.continue"] ||
        [method isEqualToString:@"network.fulfill"] ||
        [method isEqualToString:@"network.abort"]) {
        NSString *requestId = params[@"requestId"];
        if (![requestId isKindOfClass:[NSString class]] || !requestId.length) {
            if (error) {
                *error = invalid(@"missing params.requestId");
            }
            return nil;
        }
        if ([method isEqualToString:@"network.fulfill"]) {
            NSInteger status = [params[@"status"] integerValue];
            if (status < 100 || status > 599) {
                if (error) {
                    *error = invalid(@"status must be an HTTP status code");
                }
                return nil;
            }
        }
        if (params[@"body"] && ![[NSData alloc] initWithBase64EncodedString:params[@"body"] options:0]) {
            if (error) {
                *error = invalid(@"body must be base64");
            }
            return nil;
        }
        NSString *action = [method substringFromIndex:[@"network." length]];
        [stateLock lock];
        MobilePause *pause = paused[requestId];
        if (!pause || pause.verdict) {
            [stateLock unlock];
            if (error) {
                *error = invalid([NSString stringWithFormat:@"request is not paused: %@", requestId]);
            }
            return nil;
        }
        pause.verdict = @{@"action": action, @"params": params};
        [stateLock broadcast];
        [stateLock unlock];
        return @{@"status": @"ok"};
    }
    if (error) {
        *error = [NSError errorWithDomain:kErrorDomain code:-32601
            userInfo:@{NSLocalizedDescriptionKey: [NSString stringWithFormat:@"method not found: %@", method]}];
    }
    return nil;
}
@end

@implementation MobileURLProtocol {
    NSString *_requestId;
    NSURLRequest *_sourceRequest;
}

// URLSession may turn an HTTPBody into a stream before URLProtocol sees it.
// Buffer small, known-length streams so the event can report the bytes and
// continue can replay exactly what was sent. Leave large streams untouched.
static NSURLRequest *materializeSmallBody(NSURLRequest *request) {
    if (request.HTTPBody || !request.HTTPBodyStream) {
        return request;
    }
    NSString *lengthText = request.allHTTPHeaderFields[@"Content-Length"];
    NSUInteger length = (NSUInteger)[lengthText longLongValue];
    if (!length || length > kMaxEventBody) {
        return request;
    }
    NSInputStream *stream = request.HTTPBodyStream;
    [stream open];
    NSMutableData *data = [NSMutableData dataWithCapacity:length];
    uint8_t buffer[8192];
    while (data.length < length) {
        NSInteger count = [stream read:buffer maxLength:MIN(sizeof(buffer), length - data.length)];
        if (count <= 0) {
            break;
        }
        [data appendBytes:buffer length:(NSUInteger)count];
    }
    [stream close];
    if (data.length != length) {
        return request;
    }
    NSMutableURLRequest *copy = [request mutableCopy];
    copy.HTTPBody = data;
    return copy;
}

+ (BOOL)canInitWithRequest:(NSURLRequest *)request {
    if (![request.URL.scheme.lowercaseString isEqualToString:@"http"] &&
        ![request.URL.scheme.lowercaseString isEqualToString:@"https"]) {
        return NO;
    }
    if ([NSURLProtocol propertyForKey:kHandledKey inRequest:request]) {
        return NO;
    }
    return [MobileNetwork matches:request];
}

+ (BOOL)canInitWithTask:(NSURLSessionTask *)task {
    return [self canInitWithRequest:task.currentRequest ?: task.originalRequest];
}

+ (NSURLRequest *)canonicalRequestForRequest:(NSURLRequest *)request { return request; }

- (void)startLoading {
    _sourceRequest = materializeSmallBody(self.request);
    _requestId = [MobileNetwork pauseRequest:_sourceRequest];
    if (!_requestId) {
        [self forward:_sourceRequest];
        return;
    }
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
        NSDictionary *verdict = [MobileNetwork waitFor:self->_requestId];
        if (self.stopped) {
            return;
        }
        NSString *action = verdict[@"action"];
        NSDictionary *params = verdict[@"params"] ?: @{};
        if ([action isEqualToString:@"fulfill"]) {
            [self fulfill:params];
        } else if ([action isEqualToString:@"abort"]) {
            NSString *hint = params[@"errorCode"];
            NSInteger code = [hint isEqualToString:@"timedout"] ? NSURLErrorTimedOut :
                [hint isEqualToString:@"connectionrefused"] ? NSURLErrorCannotConnectToHost :
                NSURLErrorNetworkConnectionLost;
            [self.client URLProtocol:self didFailWithError:[NSError errorWithDomain:NSURLErrorDomain
                code:code userInfo:@{NSLocalizedDescriptionKey: hint ?: @"mobilecli network abort"}]];
        } else {
            NSMutableURLRequest *request = [self->_sourceRequest mutableCopy];
            if ([params[@"url"] isKindOfClass:[NSString class]]) {
                request.URL = [NSURL URLWithString:params[@"url"]];
            }
            if ([params[@"method"] isKindOfClass:[NSString class]]) {
                request.HTTPMethod = params[@"method"];
            }
            if ([params[@"headers"] isKindOfClass:[NSDictionary class]]) {
                request.allHTTPHeaderFields = params[@"headers"];
            }
            if ([params[@"body"] isKindOfClass:[NSString class]]) {
                request.HTTPBody = [[NSData alloc] initWithBase64EncodedString:params[@"body"] options:0];
            }
            [self forward:request];
        }
    });
}

- (void)forward:(NSURLRequest *)request {
    if (self.stopped) {
        return;
    }
    NSMutableURLRequest *marked = [request mutableCopy];
    [NSURLProtocol setProperty:@YES forKey:kHandledKey inRequest:marked];
    NSURLSessionConfiguration *configuration = [NSURLSessionConfiguration ephemeralSessionConfiguration];
    NSMutableArray *classes = [configuration.protocolClasses mutableCopy] ?: [NSMutableArray array];
    [classes removeObject:[MobileURLProtocol class]];
    configuration.protocolClasses = classes;
    self.forwardSession = [NSURLSession sessionWithConfiguration:configuration];
    __weak typeof(self) weakSelf = self;
    self.forwardTask = [self.forwardSession dataTaskWithRequest:marked
        completionHandler:^(NSData *data, NSURLResponse *response, NSError *error) {
            MobileURLProtocol *owner = weakSelf;
            if (!owner || owner.stopped) {
                return;
            }
            if (error) {
                [owner.client URLProtocol:owner didFailWithError:error];
            } else {
                [owner.client URLProtocol:owner didReceiveResponse:response cacheStoragePolicy:NSURLCacheStorageNotAllowed];
                if (data.length) {
                    [owner.client URLProtocol:owner didLoadData:data];
                }
                [owner.client URLProtocolDidFinishLoading:owner];
            }
            [owner.forwardSession finishTasksAndInvalidate];
        }];
    [self.forwardTask resume];
}

- (void)fulfill:(NSDictionary *)params {
    NSData *body = [params[@"body"] isKindOfClass:[NSString class]]
        ? [[NSData alloc] initWithBase64EncodedString:params[@"body"] options:0] : [NSData data];
    NSHTTPURLResponse *response = [[NSHTTPURLResponse alloc] initWithURL:self.request.URL
        statusCode:[params[@"status"] integerValue] HTTPVersion:@"HTTP/1.1"
        headerFields:[params[@"headers"] isKindOfClass:[NSDictionary class]] ? params[@"headers"] : @{}];
    [self.client URLProtocol:self didReceiveResponse:response cacheStoragePolicy:NSURLCacheStorageNotAllowed];
    if (body.length) {
        [self.client URLProtocol:self didLoadData:body];
    }
    [self.client URLProtocolDidFinishLoading:self];
}

- (void)stopLoading {
    self.stopped = YES;
    [MobileNetwork cancel:_requestId];
    [self.forwardTask cancel];
    [self.forwardSession invalidateAndCancel];
}
@end
