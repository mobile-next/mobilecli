#import "DKScreenshot.h"

#import <XCTest/XCTest.h>

#import "XCTRunnerDaemonSession.h"
#import "XCTestManager_ManagerInterface-Protocol.h"
#import "XCUIScreen.h"

@import UniformTypeIdentifiers;

// XCTImageEncoding and XCTScreenshotRequest live in XCTAutomationSupport, which the runner
// does not link, so they are looked up by name. Declaring their initializers lets ARC treat
// alloc/init the usual way. This stays Objective-C because Swift cannot alloc an unlinked
// class and pick an initializer without unsafe casts.
@protocol DKImageEncoding
- (instancetype)initWithUniformTypeIdentifier:(NSString *)uti
                           compressionQuality:(double)compressionQuality;
@end

@protocol DKScreenshotRequest
- (instancetype)initWithScreenID:(long long)screenID rect:(CGRect)rect encoding:(id)encoding;
@end

static NSError *DKScreenshotError(NSString *message) {
    return [NSError errorWithDomain:@"DKScreenshot"
                               code:1
                           userInfo:@{NSLocalizedDescriptionKey : message}];
}

static id _Nullable DKMakeJPEGScreenshotRequest(double quality) {
    Class encodingClass = NSClassFromString(@"XCTImageEncoding");
    Class requestClass = NSClassFromString(@"XCTScreenshotRequest");
    SEL encodingInit = @selector(initWithUniformTypeIdentifier:compressionQuality:);
    SEL requestInit = @selector(initWithScreenID:rect:encoding:);
    if (![encodingClass instancesRespondToSelector:encodingInit] ||
        ![requestClass instancesRespondToSelector:requestInit]) {
        return nil;
    }

    id encoding = [(id<DKImageEncoding>)[encodingClass alloc]
        initWithUniformTypeIdentifier:UTTypeJPEG.identifier
                   compressionQuality:quality];
    return [(id<DKScreenshotRequest>)[requestClass alloc]
        initWithScreenID:XCUIScreen.mainScreen.displayID
                    rect:CGRectNull
                encoding:encoding];
}

static id<XCTestManager_ManagerInterface> DKDaemonProxy(void) {
    return [XCTRunnerDaemonSession sharedSession].daemonProxy;
}

@implementation DKScreenshot

+ (nullable NSData *)captureJPEGWithQuality:(double)quality
                                    timeout:(NSTimeInterval)timeout
                                      error:(NSError **)error {
    id request = DKMakeJPEGScreenshotRequest(quality);
    if (nil == request) {
        if (error != nil) {
            *error = DKScreenshotError(@"XCTest screenshot request API is unavailable");
        }
        return nil;
    }

    // __block storage outlives this frame, so a reply arriving after the timeout is harmless
    __block NSData *jpeg = nil;
    __block NSError *replyError = nil;
    dispatch_semaphore_t replied = dispatch_semaphore_create(0);
    [DKDaemonProxy() _XCT_requestScreenshot:request
                                  withReply:^(id image, NSError *err) {
                                    replyError = err;
                                    // the reply is an XCTImage; its data property holds the encoded bytes
                                    if ([image respondsToSelector:@selector(data)]) {
                                        jpeg = [image valueForKey:@"data"];
                                    }
                                    dispatch_semaphore_signal(replied);
                                  }];

    dispatch_time_t deadline = dispatch_time(DISPATCH_TIME_NOW, (int64_t)(timeout * NSEC_PER_SEC));
    if (dispatch_semaphore_wait(replied, deadline) != 0) {
        if (error != nil) {
            *error = DKScreenshotError(@"Screenshot capture timed out");
        }
        return nil;
    }

    if (replyError != nil) {
        if (error != nil) {
            *error = replyError;
        }
        return nil;
    }

    return jpeg;
}

@end
