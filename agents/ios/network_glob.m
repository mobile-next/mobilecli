#import "network_glob.h"

NSRegularExpression *MobileCompileNetworkGlob(NSString *glob) {
    NSMutableString *pattern = [NSMutableString stringWithString:@"^"];
    NSCharacterSet *meta = [NSCharacterSet characterSetWithCharactersInString:@"\\.[]{}()+-^$|"];
    for (NSUInteger i = 0; i < glob.length; i++) {
        unichar c = [glob characterAtIndex:i];
        if (c == '*') {
            if (i + 1 < glob.length && [glob characterAtIndex:i + 1] == '*') {
                [pattern appendString:@".*"];
                i++;
            } else {
                [pattern appendString:@"[^/]*"];
            }
        } else if (c == '?') {
            [pattern appendString:@"."];
        } else {
            if ([meta characterIsMember:c]) {
                [pattern appendString:@"\\"];
            }
            [pattern appendFormat:@"%C", c];
        }
    }
    [pattern appendString:@"$"];
    return [NSRegularExpression regularExpressionWithPattern:pattern
        options:NSRegularExpressionDotMatchesLineSeparators error:nil];
}
