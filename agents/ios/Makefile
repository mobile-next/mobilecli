ARCH := arm64
MIN  := 15.0
SRCS := agent.m server.m bridge.m dispatcher.m network.m network_glob.m

FLAGS = -arch $(ARCH) \
        -dynamiclib -fPIC -fobjc-arc \
        -framework Foundation -framework UIKit -framework WebKit

all: agent-sim.dylib agent-dev.dylib

agent-sim.dylib: $(SRCS) server.h bridge.h dispatcher.h network.h network_glob.h
	xcrun -sdk iphonesimulator clang $(FLAGS) \
		-mios-simulator-version-min=$(MIN) \
		-o $@ $(SRCS)

agent-dev.dylib: $(SRCS) server.h bridge.h dispatcher.h network.h network_glob.h
	xcrun -sdk iphoneos clang $(FLAGS) \
		-miphoneos-version-min=$(MIN) \
		-o $@ $(SRCS)
	codesign -s - --force $@

clean:
	rm -f agent-sim.dylib agent-dev.dylib

test-glob:
	clang -fobjc-arc -framework Foundation -o /tmp/mobilecli-network-glob-test network_glob.m test/network_glob.m
	/tmp/mobilecli-network-glob-test

.PHONY: test-glob
