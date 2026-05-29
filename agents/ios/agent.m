#import <Foundation/Foundation.h>
#import "server.h"
#import "dispatcher.h"

static int mobilecli_port = 0;

// callable from lldb after dlopen:  expr (int)mobilecli_get_port()
__attribute__((visibility("default")))
int mobilecli_get_port(void) { return mobilecli_port; }

__attribute__((constructor))
static void on_load(void) {
    NSLog(@"[mobilecli] agent loaded");

    MobileServer *server = [[MobileServer alloc] initWithHandler:^NSData *(NSData *body) {
        return dispatch_rpc(body);
    }];

    if (![server bind]) {
        NSLog(@"[mobilecli] failed to bind server — no port available in range");
        return;
    }

    mobilecli_port = server.port;
    NSLog(@"[mobilecli] port %d (read with: expr (int)mobilecli_get_port())", mobilecli_port);

    dispatch_async(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
        [server run];
    });
}
