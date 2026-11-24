#import <Foundation/Foundation.h>

// Playwright-style URL glob: * stops at '/', ** crosses '/', and ? matches one character.
NSRegularExpression * _Nullable MobileCompileNetworkGlob(NSString * _Nonnull glob);
