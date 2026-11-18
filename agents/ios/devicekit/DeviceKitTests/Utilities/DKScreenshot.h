#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

/// Captures the main screen as JPEG through testmanagerd. The daemon encodes the image itself,
/// which is far faster than XCUIScreen.screenshot() and is what keeps MJPEG streaming smooth.
@interface DKScreenshot : NSObject

+ (nullable NSData *)captureJPEGWithQuality:(double)quality
                                    timeout:(NSTimeInterval)timeout
                                      error:(NSError *_Nullable *_Nullable)error;

@end

NS_ASSUME_NONNULL_END
