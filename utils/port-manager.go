package utils

import (
	"fmt"
	"net"
	"strings"
)

func IsPortAvailable(host string, port int) bool {
	// Verbose("Checking if port %d is available on %s", port, host)
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		// only log unexpected errors, not "address already in use"
		if !strings.Contains(err.Error(), "address already in use") {
			Verbose("error: %v", err)
		}
		return false
	}

	defer func() { _ = listener.Close() }()
	return true
}

func FindAvailablePortInRange(startPort, endPort int) (int, error) {
	for port := startPort; port <= endPort; port++ {
		if IsPortAvailable("localhost", port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", startPort, endPort)
}
