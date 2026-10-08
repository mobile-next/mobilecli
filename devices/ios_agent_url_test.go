package devices

import (
	"context"
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
	return &IOSDevice{
		Udid:            "00008110-000000000000001E",
		OSVersion:       "17.5",
		deviceKitClient: devicekit.NewDeviceKitClient(agentURL),
	}
}

func TestStartAgentUsesTheAgentURLFromTheEnvironment(t *testing.T) {
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
	t.Setenv(iosAgentURLEnv, agent.URL)

	device := deviceWithoutTunnel("http://127.0.0.1:1")

	require.NoError(t, device.StartAgent(StartAgentConfig{}))
	info, err := device.Info()
	require.NoError(t, err)
	assert.Equal(t, 393, info.ScreenSize.Width)
}

func TestStartAgentReportsAnAgentURLNothingAnswers(t *testing.T) {
	t.Setenv(iosAgentURLEnv, "http://127.0.0.1:1")

	err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no agent answering at http://127.0.0.1:1")
	assert.Contains(t, err.Error(), iosAgentURLEnv)
}

func TestStartAgentRejectsAnInvalidAgentURL(t *testing.T) {
	for _, value := range []string{"localhost:8100", "ftp://localhost:8100", "http://"} {
		t.Setenv(iosAgentURLEnv, value)

		err := deviceWithoutTunnel("http://127.0.0.1:1").StartAgent(StartAgentConfig{})

		require.Error(t, err, value)
		assert.Contains(t, err.Error(), "is not a valid agent URL", value)
	}
}

func TestMjpegURLFollowsTheAgentURL(t *testing.T) {
	assert.Equal(t, "http://192.168.1.20:8100/mjpeg?fps=10", buildMjpegURLForAgent("http://192.168.1.20:8100", 10, 1))
	assert.Equal(t, "http://[fd34:13d9:5468::1]:12004/mjpeg?fps=10", buildMjpegURLForAgent("http://[fd34:13d9:5468::1]:12004", 10, 1))
	assert.Equal(t, "http://localhost:8100/mjpeg?scale=50", buildMjpegURL(8100, 0, 0.5))
}
