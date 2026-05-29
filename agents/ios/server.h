#import <Foundation/Foundation.h>

typedef NSData * _Nullable (^RpcHandler)(NSData * _Nonnull body);

@interface MobileServer : NSObject
- (instancetype _Nonnull)initWithHandler:(RpcHandler _Nonnull)handler;
@property (readonly) int port;
- (BOOL)bind;       // binds and listens; call once, returns immediately
- (void)run;        // accept loop — blocks the calling thread
@end
