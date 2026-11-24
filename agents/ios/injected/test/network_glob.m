#import <Foundation/Foundation.h>
#import "../network_glob.h"

static BOOL matches(NSString *glob, NSString *url) {
    NSRegularExpression *pattern = MobileCompileNetworkGlob(glob);
    return [pattern numberOfMatchesInString:url options:0 range:NSMakeRange(0, url.length)] == 1;
}

int main(void) {
    @autoreleasepool {
        NSCAssert(matches(@"**/todos/**", @"https://example.test/todos/1"), @"double star must cross slashes");
        NSCAssert(matches(@"https://*.test/todos/?", @"https://api.test/todos/1"), @"star and question mark");
        NSCAssert(!matches(@"https://*.test/todos/?", @"https://api.test/todos/12"), @"question mark is one character");
        NSCAssert(!matches(@"https://*.test/todos/?", @"https://api.test/deep/todos/1"), @"single star stops at slash");
        NSCAssert(matches(@"https://api.test/a.b", @"https://api.test/a.b"), @"dot is literal");
        NSCAssert(!matches(@"https://api.test/a.b", @"https://api.test/axb"), @"dot is not regex wildcard");
    }
    return 0;
}
