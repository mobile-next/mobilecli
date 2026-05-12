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

CC_ANDROID  := $(TOOLCHAIN)/aarch64-linux-android26-clang
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

.PHONY: all clean

all: devicekit.so devicekit.dex

devicekit.so: jvmti_agent.c
	$(CC_ANDROID) -shared -fPIC -O2 \
		-I$(SYSROOT) -I$(JDK_INC) -I$(JDK_INC)/$(JDK_PLATFORM) \
		-o $@ $< -llog

devicekit.dex: $(JAVA_SRCS)
	mkdir -p .dex_build
	$(JAVAC) --release 8 -cp $(ANDROID_JAR) -d .dex_build $(JAVA_SRCS)
	$(D8) --min-api 26 --output .dex_build $$(find .dex_build -name "*.class")
	mv .dex_build/classes.dex devicekit.dex
	rm -rf .dex_build

clean:
	rm -f devicekit.so devicekit.dex
	rm -rf .dex_build
