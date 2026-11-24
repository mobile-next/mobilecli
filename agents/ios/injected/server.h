#import <Foundation/Foundation.h>

typedef NSData * _Nullable (^RpcHandler)(NSData * _Nonnull body);

@interface MobileServer : NSObject
- (instancetype _Nonnull)initWithHandler:(RpcHandler _Nonnull)handler;
@property (readonly) int port;
+ (int)activePort;  // port the app's server listens on, 0 if none; read by the debugger after the CLI lost it
- (BOOL)bind;       // binds and listens; call once, returns immediately
- (void)run;        // accept loop — blocks the calling thread
@end
