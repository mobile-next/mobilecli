NDK_VERSION         ?= 29.0.13113456
BUILD_TOOLS_VERSION ?= 36.0.0

# macOS default; override with ANDROID_HOME env var (e.g. on Linux CI)
ANDROID_HOME ?= $(HOME)/Library/Android/sdk

# Detect host OS for NDK toolchain path and JNI header subdir
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
  PREBUILT     := linux-x86_64
  JDK_PLATFORM := linux
else
  PREBUILT     := darwin-x86_64
  JDK_PLATFORM := darwin
endif

NDK       := $(ANDROID_HOME)/ndk/$(NDK_VERSION)
TOOLCHAIN := $(NDK)/toolchains/llvm/prebuilt/$(PREBUILT)/bin
SYSROOT   := $(NDK)/toolchains/llvm/prebuilt/$(PREBUILT)/sysroot/usr/include

# the agent is loaded into the app process, so it is built for each ABI an app
# can run as: arm64-v8a (devices, arm64 emulators) and x86_64 (x86_64 emulators)
CC_arm64-v8a := $(TOOLCHAIN)/aarch64-linux-android26-clang
CC_x86_64    := $(TOOLCHAIN)/x86_64-linux-android26-clang

D8          := $(ANDROID_HOME)/build-tools/$(BUILD_TOOLS_VERSION)/d8
ANDROID_JAR := $(ANDROID_HOME)/platforms/android-35/android.jar

# Java: prefer JAVA_HOME, then macOS java_home helper, then system javac
ifndef JAVA_HOME
  JAVA_HOME := $(shell /usr/libexec/java_home 2>/dev/null)
endif
ifeq ($(JAVA_HOME),)
  JAVAC   := $(shell which javac)
  JDK_INC := $(shell dirname $$(dirname $$(which javac)))/include
else
  JAVAC   := $(JAVA_HOME)/bin/javac
  JDK_INC := $(JAVA_HOME)/include
endif

JAVA_SRCS := $(wildcard java/*.java)
OUTPUT_DIR := ../../internal/agentassets/data/android

.PHONY: all clean lint

all: $(OUTPUT_DIR)/mobilecli-arm64-v8a.so $(OUTPUT_DIR)/mobilecli-x86_64.so $(OUTPUT_DIR)/mobilecli.dex

$(OUTPUT_DIR):
	mkdir -p $@

$(OUTPUT_DIR)/mobilecli-%.so: jvmti_agent.c | $(OUTPUT_DIR)
	$(CC_$*) -shared -fPIC -O2 \
		-I$(SYSROOT) -I$(JDK_INC) -I$(JDK_INC)/$(JDK_PLATFORM) \
		-o $@ $< -llog

$(OUTPUT_DIR)/mobilecli.dex: $(JAVA_SRCS) | $(OUTPUT_DIR)
	mkdir -p .dex_build
	$(JAVAC) --release 8 -Xlint:all,-options -Werror -cp $(ANDROID_JAR) -d .dex_build $(JAVA_SRCS)
	$(D8) --min-api 26 --output .dex_build $$(find .dex_build -name "*.class")
	mv .dex_build/classes.dex $@
	rm -rf .dex_build

# compile-only pass with -Xlint -Werror; no dex, no NDK needed
lint:
	$(JAVAC) --release 8 -Xlint:all,-options -Werror -cp $(ANDROID_JAR) -d $(shell mktemp -d) $(JAVA_SRCS)

clean:
	rm -f $(OUTPUT_DIR)/mobilecli-*.so $(OUTPUT_DIR)/mobilecli.dex
	rm -rf .dex_build
