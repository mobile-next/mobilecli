package devices

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleCrashLog = `--------- beginning of crash
2026-03-02 10:58:30.938  1300  1440 F libc    : Fatal signal 6 (SIGABRT), code -1 (SI_QUEUE) in tid 1440 (bt_stack_manage), pid 1300 (droid.bluetooth)
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : *** *** *** *** *** *** *** *** *** *** *** *** *** *** *** ***
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Build fingerprint: 'google/sdk_gphone64_arm64/emu64a:16/BE2A.250530.026.D1/13818094:user/release-keys'
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Revision: '0'
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : ABI: 'arm64'
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Timestamp: 2026-03-02 10:58:31.456812658+0100
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Process uptime: 0s
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Cmdline: com.google.android.bluetooth
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : pid: 1300, tid: 1440, name: bt_stack_manage  >>> com.google.android.bluetooth <<<
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : uid: 1002
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : signal 6 (SIGABRT), code -1 (SI_QUEUE), fault addr --------
2026-03-02 10:58:32.108 11874 11874 F DEBUG   : Abort message: 'system/gd/hci/hci_layer.cc:527 on_hardware_error: Hardware Error Event with code 0x42'
2026-03-02 10:58:32.108 11874 11874 F DEBUG   :       #00 pc 00000000000707b0  /apex/com.android.runtime/lib64/bionic/libc.so (abort+156)
2026-03-02 10:58:32.108 11874 11874 F DEBUG   :       #01 pc 00000000008a1534  /apex/com.android.art/lib64/libart.so (art::Runtime::Abort(char const*)+620)
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: FATAL EXCEPTION: main
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: PID: 13066
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: java.lang.ArrayIndexOutOfBoundsException: length=1; index=2
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: 	at com.mobilenext.devicekit.CrashSimulator.main(Unknown Source:9)
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: 	at com.android.internal.os.RuntimeInit.nativeFinishInit(Native Method)
2026-03-27 14:23:18.378 13066 13066 E AndroidRuntime: 	at com.android.internal.os.RuntimeInit.main(RuntimeInit.java:371)
2026-03-27 14:23:18.386 13066 13066 E AndroidRuntime: Couldn't report crash. Here's the crash:
2026-03-27 14:23:18.386 13066 13066 E AndroidRuntime: java.lang.ArrayIndexOutOfBoundsException: length=1; index=2
2026-03-27 14:23:18.386 13066 13066 E AndroidRuntime: 	at com.mobilenext.devicekit.CrashSimulator.main(Unknown Source:9)
2026-03-27 14:23:18.386 13066 13066 E AndroidRuntime: 	at com.android.internal.os.RuntimeInit.nativeFinishInit(Native Method)
2026-03-27 14:23:18.386 13066 13066 E AndroidRuntime: 	at com.android.internal.os.RuntimeInit.main(RuntimeInit.java:371)`

func TestParseAndroidCrashLogFindsNativeAndJavaCrashes(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	require.Len(t, crashes, 2, "should find exactly 2 crashes (1 native + 1 java)")
}

func TestNativeCrashHasCorrectProcessName(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	assert.Equal(t, "com.google.android.bluetooth", crashes[0].ProcessName)
}

func TestNativeCrashHasCorrectTimestamp(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	assert.Equal(t, "2026-03-02 10:58:32.108", crashes[0].Timestamp)
}

func TestNativeCrashIDContainsCrashingPID(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	// id should use the crashing process pid (1300), not the debuggerd pid (11874)
	assert.Equal(t, "2026-03-02_10:58:32.108_1300", crashes[0].ID)
}

func TestJavaCrashExtractsProcessFromStackTrace(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	assert.Equal(t, "com.mobilenext.devicekit.CrashSimulator", crashes[1].ProcessName)
}

func TestJavaCrashHasCorrectTimestamp(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	assert.Equal(t, "2026-03-27 14:23:18.378", crashes[1].Timestamp)
}

func TestJavaCrashIDContainsPID(t *testing.T) {
	crashes := ParseAndroidCrashLog(sampleCrashLog)

	assert.Equal(t, "2026-03-27_14:23:18.378_13066", crashes[1].ID)
}

func TestJavaCrashWithProcessLine(t *testing.T) {
	log := `2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: FATAL EXCEPTION: main
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: Process: com.example.myapp, PID: 12345
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: java.lang.NullPointerException
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: 	at com.example.myapp.MainActivity.onCreate(MainActivity.java:42)`

	crashes := ParseAndroidCrashLog(log)

	require.Len(t, crashes, 1)
	assert.Equal(t, "com.example.myapp", crashes[0].ProcessName)
}

func TestExtractAndroidCrashReturnsFullNativeCrash(t *testing.T) {
	content, err := ExtractAndroidCrash(sampleCrashLog, "2026-03-02_10:58:32.108_1300")

	require.NoError(t, err)
	assert.Contains(t, content, "com.google.android.bluetooth")
	assert.Contains(t, content, "*** ***")
	assert.Contains(t, content, "abort+156")
}

func TestExtractAndroidCrashReturnsFullJavaCrash(t *testing.T) {
	content, err := ExtractAndroidCrash(sampleCrashLog, "2026-03-27_14:23:18.378_13066")

	require.NoError(t, err)
	assert.Contains(t, content, "FATAL EXCEPTION")
	assert.Contains(t, content, "ArrayIndexOutOfBoundsException")
	// should include the "Couldn't report crash" follow-up lines too
	assert.Contains(t, content, "Couldn't report crash")
}

func TestExtractAndroidCrashReturnsErrorForUnknownID(t *testing.T) {
	_, err := ExtractAndroidCrash(sampleCrashLog, "9999-99-99_99:99:99.999_0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEmptyLogReturnsEmptyArray(t *testing.T) {
	crashes := ParseAndroidCrashLog("")

	assert.Empty(t, crashes)
	assert.NotNil(t, crashes)
}

func TestLogWithNoCrashesReturnsEmptyArray(t *testing.T) {
	log := `2026-03-27 14:00:00.000  1000  1000 I SomeTag : just a normal log line
2026-03-27 14:00:01.000  1000  1000 D AnotherTag : nothing to see here`

	crashes := ParseAndroidCrashLog(log)

	assert.Empty(t, crashes)
}

func TestJavaCrashWithInnerClassFrame(t *testing.T) {
	log := `2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: FATAL EXCEPTION: main
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: java.lang.NullPointerException
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: 	at com.example.myapp.MainActivity$1.onClick(MainActivity.java:42)`

	crashes := ParseAndroidCrashLog(log)

	require.Len(t, crashes, 1)
	assert.Equal(t, "com.example.myapp.MainActivity$1", crashes[0].ProcessName)
}

func TestJavaCrashWithLambdaFrame(t *testing.T) {
	log := `2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: FATAL EXCEPTION: main
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: java.lang.NullPointerException
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: 	at com.example.myapp.$$ExternalSyntheticLambda0.run(Unknown Source:4)`

	crashes := ParseAndroidCrashLog(log)

	require.Len(t, crashes, 1)
	assert.Equal(t, "com.example.myapp.$$ExternalSyntheticLambda0", crashes[0].ProcessName)
}

func TestJavaCrashWithConstructorFrame(t *testing.T) {
	log := `2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: FATAL EXCEPTION: main
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: java.lang.NullPointerException
2026-03-27 14:00:00.000 12345 12345 E AndroidRuntime: 	at com.example.myapp.MyService.<init>(MyService.java:15)`

	crashes := ParseAndroidCrashLog(log)

	require.Len(t, crashes, 1)
	assert.Equal(t, "com.example.myapp.MyService", crashes[0].ProcessName)
}

func TestExtractedNativeCrashDoesNotIncludeJavaCrash(t *testing.T) {
	content, err := ExtractAndroidCrash(sampleCrashLog, "2026-03-02_10:58:32.108_1300")

	require.NoError(t, err)
	assert.False(t, strings.Contains(content, "AndroidRuntime"))
}
