#import <Foundation/Foundation.h>

// Shared request state and URLSession hook for the simulator agent.
@interface MobileNetwork : NSObject
+ (void)install;
+ (nullable NSDictionary *)perform:(NSString * _Nonnull)method
                            params:(nullable NSDictionary *)params
                             error:(NSError * _Nullable * _Nullable)error;
@end
