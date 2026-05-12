ARCH := arm64
MIN  := 15.0
SRCS := agent.m server.m bridge.m dispatcher.m

FLAGS = -arch $(ARCH) \
        -dynamiclib -fPIC -fobjc-arc \
        -framework Foundation -framework UIKit -framework WebKit

all: agent-sim.dylib

agent-sim.dylib: $(SRCS) server.h bridge.h dispatcher.h
	xcrun -sdk iphonesimulator clang $(FLAGS) \
		-mios-simulator-version-min=$(MIN) \
		-o $@ $(SRCS)

agent-dev.dylib: $(SRCS) server.h bridge.h dispatcher.h
	xcrun -sdk iphoneos clang $(FLAGS) \
		-miphoneos-version-min=$(MIN) \
		-o $@ $(SRCS)

# legacy test dylib
devicekit.dylib: devicekit.m
	xcrun -sdk iphonesimulator clang -arch $(ARCH) \
		-mios-simulator-version-min=$(MIN) \
		-dynamiclib -fPIC -fobjc-arc \
		-framework Foundation \
		-o $@ $<

clean:
	rm -f agent-sim.dylib agent-dev.dylib devicekit.dylib
