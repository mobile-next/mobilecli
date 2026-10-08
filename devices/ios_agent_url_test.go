package devices

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mobile-next/mobilecli/devices/devicekit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAgentHandler answers /health and device.info the way devicekit-ios does.
func fakeAgentHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"scale":3,"screenSize":{"width":393,"height":852}}}`))
	})
	return mux
}

func fakeAgent(t *testing.T) *httptest.Server {
	agent := httptest.NewServer(fakeAgentHandler())
	t.Cleanup(agent.Close)
	return agent
}

// an iOS 17 device with no tunnel manager: any attempt to start a tunnel,
// forward a port or list apps would fail instead of reaching the fake agent.
func deviceWithoutTunnel(agentURL string) *IOSDevice {
	return deviceWithUdid("00008110-000000000000001E", agentURL)
}

func deviceWithUdid(udid, agentURL string) *IOSDevice {
	return &IOSDevice{
		Udid:            udid,
		OSVersion:       "17.5",
		deviceKitClient: devicekit.NewDeviceKitClient(agentURL),
	}
}

// withConnectedIOSDevices makes the bare URL check see count devices instead of
// asking usbmuxd.
func withConnectedIOSDevices(t *testing.T, count int) {
	original := countConnectedIOSDevices
	countConnectedIOSDevices = func() (int, error) { return count, nil }
	t.Cleanup(func() { countConnectedIOSDevices = original })
}

func TestStartAgentUsesTheAgentURLFromTheEnvironment(t *testing.T) {
	withConnectedIOSDevices(t, 1)
	agent := fakeAgent(t)
	t.Setenv(iosAgentURLEnv, agent.URL+"/")

	// the default client points somewhere nothing listens
	device := deviceWithoutTunnel("http://127.0.0.1:1")

	require.NoError(t, device.StartAgent(StartAgentConfig{}))
	assert.Equal(t, agent.URL, device.deviceKitClient.BaseURL())

	info, err := device.Info()
	require.NoError(t, err)
	assert.Equal(t, 393, info.ScreenSize.Width)
	assert.Equal(t, 852, info.ScreenSize.Height)
}

// the Xcode CoreDevice tunnel address is IPv6, so the URL carries a bracketed literal
func TestStartAgentAcceptsABracketedIPv6AgentURL(t *testing.T) {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback: %v", err)
	}
	agent := httptest.NewUnstartedServer(fakeAgentHandler())
	agent.Listener = listener
	agent.Start()
	t.Cleanup(agent.Close)
	require.Contains(t, agent.URL, "[::1]")
	withConnectedIOSDevices(t, 1)
	t.Setenv(iosAgentURLEnv, agent.URL)

	device := deviceWithoutTunnel("http://127.0.0.1:1")

	require.NoError(t, device.StartAgent(StartAgentConfig{}))
	info, err := device.Info()
	require.NoError(t, err)
	assert.Equal(t, 393, info.ScreenSize.Width)
}

func TestStartAgentReportsAnAgentURLNothingAnswers(t *testing.T) {
	withConnectedIOSDevices(t, 1)
	t.Setenv(iosAgentURLEnv, "http://127.0.0.1:1")

	err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no agent answering at http://127.0.0.1:1")
	assert.Contains(t, err.Error(), iosAgentURLEnv)
}

func TestStartAgentRejectsAnInvalidAgentURL(t *testing.T) {
	withConnectedIOSDevices(t, 1)
	for _, value := range []string{"localhost:8100", "ftp://localhost:8100", "http://"} {
		t.Setenv(iosAgentURLEnv, value)

		err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

		require.Error(t, err, value)
		assert.Contains(t, err.Error(), "is not a valid agent URL", value)
	}
}

// /health and /rpc are appended to the agent URL, so a query or fragment would
// swallow them, and the agent has no authentication to send credentials to.
func TestStartAgentRejectsAQueryFragmentOrUserInfo(t *testing.T) {
	withConnectedIOSDevices(t, 1)
	cases := map[string]string{
		"http://localhost:8100?x":                   "cannot have a query or fragment",
		"http://localhost:8100/?":                   "cannot have a query or fragment",
		"http://localhost:8100/#":                   "cannot have a query or fragment",
		"http://localhost:8100/#health":             "cannot have a query or fragment",
		"http://user:secret@192.168.1.20:8100":      "cannot carry a user name or password",
		"https://user@localhost:8100":               "cannot carry a user name or password",
		"00008110-000000000000001E=http://u:p@h:81": "cannot carry a user name or password",
		"00008110-000000000000001E=http://h:81/#x":  "cannot have a query or fragment",
	}
	for value, message := range cases {
		t.Setenv(iosAgentURLEnv, value)

		err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

		require.Error(t, err, value)
		assert.Contains(t, err.Error(), message, value)
		assert.Contains(t, err.Error(), iosAgentURLEnv, value)
	}
}

// a bare URL with a query looks like a mapping and is reported as malformed
func TestStartAgentRejectsMalformedMappings(t *testing.T) {
	for _, value := range []string{
		"http://localhost:8100?a=b",
		"00008110-000000000000001E=",
		"=http://localhost:8100",
		"00008110-000000000000001E=http://localhost:8100,",
		"00008110-000000000000001E http://localhost:8100,00008101-00161CEC3CDB001E=http://localhost:8101",
	} {
		t.Setenv(iosAgentURLEnv, value)

		err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

		require.Error(t, err, value)
		assert.Contains(t, err.Error(), "expected <udid>=<url>", value)
		assert.Contains(t, err.Error(), iosAgentURLEnv, value)
	}
}

func TestStartAgentRejectsADeviceListedTwice(t *testing.T) {
	t.Setenv(iosAgentURLEnv, "00008110-000000000000001E=http://localhost:8100,00008110-000000000000001e=http://localhost:8101")

	err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "more than once")
}

func TestStartAgentPicksTheDevicesOwnEntry(t *testing.T) {
	// the mapping is used as is, without asking usbmuxd how many devices there are
	withConnectedIOSDevices(t, 5)
	mine := fakeAgent(t)
	other := fakeAgent(t)
	t.Setenv(iosAgentURLEnv, "00008101-00161CEC3CDB001E="+other.URL+", 00008110-000000000000001e = "+mine.URL+"/")

	device := deviceWithoutTunnel("http://127.0.0.1:1")

	require.NoError(t, device.StartAgent(StartAgentConfig{}))
	assert.Equal(t, mine.URL, device.deviceKitClient.BaseURL())
}

func TestStartAgentAcceptsAnIPv6AgentURLInAMapping(t *testing.T) {
	t.Setenv(iosAgentURLEnv, "00008101-00161CEC3CDB001E=http://[fd34:13d9:5468::1]:12004,00008110-000000000000001E=http://[fd34:13d9:5468::2]:12004")

	agentURL, err := iosAgentURLOverride("00008101-00161CEC3CDB001E")

	require.NoError(t, err)
	assert.Equal(t, "http://[fd34:13d9:5468::1]:12004", agentURL)
}

func TestStartAgentTakesTheNormalPathForAnUnmappedDevice(t *testing.T) {
	// the device's own agent already answers on the normal path, the mapped
	// URL belongs to another device and nothing listens there
	running := fakeAgent(t)
	t.Setenv(iosAgentURLEnv, "00008101-00161CEC3CDB001E=http://127.0.0.1:1")

	device := deviceWithoutTunnel(running.URL)

	require.NoError(t, device.StartAgent(StartAgentConfig{}))
	assert.Equal(t, running.URL, device.deviceKitClient.BaseURL())
}

func TestStartAgentRejectsABareURLUnlessExactlyOneDeviceIsConnected(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprintf("%d devices", count), func(t *testing.T) {
			withConnectedIOSDevices(t, count)
			agent := fakeAgent(t)
			t.Setenv(iosAgentURLEnv, agent.URL)

			err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("found %d", count))
			assert.Contains(t, err.Error(), "<udid>=<url>")
			assert.Contains(t, err.Error(), iosAgentURLEnv)
		})
	}
}

func TestMjpegURLFollowsTheAgentURL(t *testing.T) {
	assert.Equal(t, "http://192.168.1.20:8100/mjpeg?fps=10", buildMjpegURLForAgent("http://192.168.1.20:8100", 10, 1))
	assert.Equal(t, "http://[fd34:13d9:5468::1]:12004/mjpeg?fps=10", buildMjpegURLForAgent("http://[fd34:13d9:5468::1]:12004", 10, 1))
	assert.Equal(t, "http://localhost:8100/mjpeg?scale=50", buildMjpegURL(8100, 0, 0.5))
}
