#import <Foundation/Foundation.h>
#import <UIKit/UIKit.h>

NS_ASSUME_NONNULL_BEGIN

@interface IosBridge : NSObject
+ (void)runOnMainThread:(dispatch_block_t)block;
+ (NSArray<NSDictionary *> *)listWebViews;
+ (nullable UIView *)webViewWithID:(NSString *)wvId;
+ (NSDictionary *)evaluateJS:(NSString *)expression inWebView:(UIView *)webView;
+ (void)gotoURL:(NSString *)urlStr inWebView:(UIView *)webView;
+ (void)reloadWebView:(UIView *)webView;
+ (void)goBackWebView:(UIView *)webView;
+ (void)goForwardWebView:(UIView *)webView;
@end

NS_ASSUME_NONNULL_END
