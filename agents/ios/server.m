#import "server.h"
#import <sys/socket.h>
#import <netinet/in.h>
#import <unistd.h>

#define BASE_PORT 27042
#define PORT_RANGE 10

@implementation MobileServer {
    RpcHandler _handler;
    int _serverFd;
    int _port;
}

- (instancetype)initWithHandler:(RpcHandler)handler {
    self = [super init];
    _handler = [handler copy];
    _serverFd = -1;
    _port = 0;
    return self;
}

- (int)port { return _port; }

- (BOOL)bindPort {
    _serverFd = socket(AF_INET, SOCK_STREAM, 0);
    if (_serverFd < 0) return NO;

    int yes = 1;
    setsockopt(_serverFd, SOL_SOCKET, SO_REUSEADDR, &yes, sizeof(yes));

    struct sockaddr_in addr = {0};
    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = htonl(INADDR_LOOPBACK);

    for (int p = BASE_PORT; p < BASE_PORT + PORT_RANGE; p++) {
        addr.sin_port = htons((uint16_t)p);
        if (bind(_serverFd, (struct sockaddr *)&addr, sizeof(addr)) == 0) {
            _port = p;
            return YES;
        }
    }
    close(_serverFd);
    _serverFd = -1;
    return NO;
}

- (BOOL)bind {
    if (![self bindPort]) {
        NSLog(@"[mobilecli] failed to bind on ports %d-%d", BASE_PORT, BASE_PORT + PORT_RANGE - 1);
        return NO;
    }
    if (listen(_serverFd, 8) < 0) {
        NSLog(@"[mobilecli] listen failed");
        return NO;
    }
    NSLog(@"[mobilecli] bound to 127.0.0.1:%d", _port);
    return YES;
}

- (void)run {
    while (1) {
        int clientFd = accept(_serverFd, NULL, NULL);
        if (clientFd < 0) continue;
        dispatch_async(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
            [self handleConnection:clientFd];
        });
    }
}

- (void)handleConnection:(int)fd {
    NSMutableData *buf = [NSMutableData data];
    char tmp[4096];
    NSData *separator = [@"\r\n\r\n" dataUsingEncoding:NSASCIIStringEncoding];
    NSRange headerEnd = {NSNotFound, 0};

    // read until the header/body separator is found
    while (headerEnd.location == NSNotFound) {
        ssize_t n = recv(fd, tmp, sizeof(tmp), 0);
        if (n <= 0) { close(fd); return; }
        [buf appendBytes:tmp length:(NSUInteger)n];
        headerEnd = [buf rangeOfData:separator options:0 range:NSMakeRange(0, buf.length)];
    }

    NSString *headerStr = [[NSString alloc] initWithData:[buf subdataWithRange:NSMakeRange(0, headerEnd.location)]
                                                encoding:NSASCIIStringEncoding];
    NSInteger contentLength = 0;
    for (NSString *line in [headerStr componentsSeparatedByString:@"\r\n"]) {
        if ([line.lowercaseString hasPrefix:@"content-length:"]) {
            contentLength = [[line substringFromIndex:15] integerValue];
        }
    }
    if (contentLength <= 0) { close(fd); return; }

    NSUInteger bodyStart = headerEnd.location + 4;
    NSMutableData *body = [NSMutableData dataWithData:[buf subdataWithRange:NSMakeRange(bodyStart, buf.length - bodyStart)]];

    while ((NSInteger)body.length < contentLength) {
        NSInteger remaining = contentLength - (NSInteger)body.length;
        ssize_t n = recv(fd, tmp, (size_t)MIN((NSInteger)sizeof(tmp), remaining), 0);
        if (n <= 0) break;
        [body appendBytes:tmp length:(NSUInteger)n];
    }

    NSData *response = _handler(body);
    if (!response) response = [@"{}" dataUsingEncoding:NSUTF8StringEncoding];

    NSString *headers = [NSString stringWithFormat:
        @"HTTP/1.1 200 OK\r\n"
        @"Content-Type: application/json\r\n"
        @"Content-Length: %lu\r\n"
        @"Connection: close\r\n"
        @"\r\n",
        (unsigned long)response.length];

    NSData *headerData = [headers dataUsingEncoding:NSASCIIStringEncoding];
    send(fd, headerData.bytes, headerData.length, 0);
    send(fd, response.bytes, response.length, 0);
    close(fd);
}

@end
