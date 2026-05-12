NDK         := $(HOME)/Library/Android/sdk/ndk/29.0.13113456
TOOLCHAIN   := $(NDK)/toolchains/llvm/prebuilt/darwin-x86_64/bin
CC_ANDROID  := $(TOOLCHAIN)/aarch64-linux-android26-clang
JDK_INC     := $(shell brew --prefix openjdk@21)/include
SYSROOT     := $(NDK)/toolchains/llvm/prebuilt/darwin-x86_64/sysroot/usr/include
JAVAC       := $(shell brew --prefix openjdk@21)/bin/javac
D8          := $(HOME)/Library/Android/sdk/build-tools/36.0.0/d8
ANDROID_JAR := $(HOME)/Library/Android/sdk/platforms/android-35/android.jar

JAVA_SRCS := $(wildcard java/*.java)

all: devicekit.so devicekit.dex

devicekit.so: jvmti_agent.c
	$(CC_ANDROID) -shared -fPIC -O2 \
		-I$(SYSROOT) -I$(JDK_INC) -I$(JDK_INC)/darwin \
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
