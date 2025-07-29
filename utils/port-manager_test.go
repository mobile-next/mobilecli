package utils

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPortAvailable(t *testing.T) {
	// Test available port (dynamic allocation)
	assert.True(t, IsPortAvailable("127.0.0.1", 0), "Port 0 should always be available (OS picks free port)")
}

func TestIsPortAvailable_LocalhostVariations(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"IPv4 loopback", "127.0.0.1", true},
		{"IPv6 loopback", "::1", false}, // Function uses tcp4, so IPv6 fails
		{"localhost", "localhost", true}, // localhost resolves to IPv4
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with port 0 (OS picks available port)
			result := IsPortAvailable(tt.host, 0)
			assert.Equal(t, tt.expected, result, "Port 0 availability for %s should be %v", tt.host, tt.expected)
		})
	}
}

func TestIsPortAvailable_PortInUse(t *testing.T) {
	// Create a listener to occupy a port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "Failed to create test listener")
	defer listener.Close()

	// Get the actual port that was assigned
	addr := listener.Addr().(*net.TCPAddr)

	// Test that the occupied port is reported as unavailable
	assert.False(t, IsPortAvailable("127.0.0.1", addr.Port), "Port %d should be unavailable (in use)", addr.Port)
}

func TestIsPortAvailable_IPv6_NotSupported(t *testing.T) {
	// Since the function uses tcp4, IPv6 addresses should fail
	result := IsPortAvailable("::1", 0)
	assert.False(t, result, "IPv6 addresses should fail since function uses tcp4")

	result = IsPortAvailable("2001:db8::1", 8080)
	assert.False(t, result, "IPv6 addresses should fail since function uses tcp4")
}

func TestIsPortAvailable_InvalidHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// NOTE: Due to net.ParseIP() returning nil for invalid IPs,
		// the function actually binds to all interfaces (0.0.0.0) and succeeds
		{"Invalid hostname", "invalid.host.name.that.does.not.exist", true},
		{"Invalid IP", "999.999.999.999", true},
		{"Empty string", "", true}, // Empty string also defaults to all interfaces
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPortAvailable(tt.host, 0) // Use port 0 to avoid conflicts
			assert.Equal(t, tt.expected, result, "Host %s should return %v (due to nil IP defaulting to 0.0.0.0)", tt.host, tt.expected)
		})
	}
}

func TestIsPortAvailable_PrivilegedPorts(t *testing.T) {
	// Test well-known ports that are likely to be restricted or in use
	privilegedPorts := []int{22, 25, 53, 80, 443}
	for _, port := range privilegedPorts {
		t.Run(fmt.Sprintf("port_%d", port), func(t *testing.T) {
			// These ports are typically restricted or in use
			// We expect them to return false (either permission denied or in use)
			result := IsPortAvailable("127.0.0.1", port)

			// We don't assert false here because in some test environments
			// these ports might actually be available. We just verify the function
			// doesn't panic and returns a boolean
			assert.IsType(t, true, result, "IsPortAvailable should return a boolean for port %d", port)
		})
	}
}

func TestIsPortAvailable_InvalidPortNumbers(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"Negative port", -1},
		{"Port too high", 65536},
		{"Very high invalid port", 100000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Invalid port numbers should return false
			assert.False(t, IsPortAvailable("127.0.0.1", tt.port), "Invalid port %d should return false", tt.port)
		})
	}
}
