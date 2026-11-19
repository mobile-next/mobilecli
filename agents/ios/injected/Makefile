# simulator apps can run as x86_64 under Rosetta (e.g. apps linking MLKit)
SIM_ARCHS := -arch arm64 -arch x86_64
DEV_ARCHS := -arch arm64
MIN  := 15.0
SRCS := agent.m server.m bridge.m dispatcher.m
OUTPUT_DIR := ../../../internal/agentassets/data/ios

FLAGS = -dynamiclib -fPIC -fobjc-arc \
        -framework Foundation -framework UIKit -framework WebKit

.PHONY: all clean

all: $(OUTPUT_DIR)/agent-sim.dylib $(OUTPUT_DIR)/agent-dev.dylib

$(OUTPUT_DIR):
	mkdir -p $@

$(OUTPUT_DIR)/agent-sim.dylib: $(SRCS) server.h bridge.h dispatcher.h Makefile | $(OUTPUT_DIR)
	xcrun -sdk iphonesimulator clang $(SIM_ARCHS) $(FLAGS) \
		-mios-simulator-version-min=$(MIN) \
		-o $@ $(SRCS)

$(OUTPUT_DIR)/agent-dev.dylib: $(SRCS) server.h bridge.h dispatcher.h Makefile | $(OUTPUT_DIR)
	xcrun -sdk iphoneos clang $(DEV_ARCHS) $(FLAGS) \
		-miphoneos-version-min=$(MIN) \
		-o $@ $(SRCS)
	codesign -s - --force $@

clean:
	rm -f $(OUTPUT_DIR)/agent-sim.dylib $(OUTPUT_DIR)/agent-dev.dylib
