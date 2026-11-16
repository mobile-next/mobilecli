ARCH := arm64
MIN  := 15.0
SRCS := agent.m server.m bridge.m dispatcher.m
OUTPUT_DIR := ../../../internal/agentassets/data/ios

FLAGS = -arch $(ARCH) \
        -dynamiclib -fPIC -fobjc-arc \
        -framework Foundation -framework UIKit -framework WebKit

.PHONY: all clean

all: $(OUTPUT_DIR)/agent-sim.dylib $(OUTPUT_DIR)/agent-dev.dylib

$(OUTPUT_DIR):
	mkdir -p $@

$(OUTPUT_DIR)/agent-sim.dylib: $(SRCS) server.h bridge.h dispatcher.h | $(OUTPUT_DIR)
	xcrun -sdk iphonesimulator clang $(FLAGS) \
		-mios-simulator-version-min=$(MIN) \
		-o $@ $(SRCS)

$(OUTPUT_DIR)/agent-dev.dylib: $(SRCS) server.h bridge.h dispatcher.h | $(OUTPUT_DIR)
	xcrun -sdk iphoneos clang $(FLAGS) \
		-miphoneos-version-min=$(MIN) \
		-o $@ $(SRCS)
	codesign -s - --force $@

clean:
	rm -f $(OUTPUT_DIR)/agent-sim.dylib $(OUTPUT_DIR)/agent-dev.dylib
