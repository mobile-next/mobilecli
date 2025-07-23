package utils

import (
	"net"
)

func IsPortAvailable(host string, port int) bool {
	Verbose("Checking if port %d is available on %s", port, host)
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP(host), Port: port})
	if err != nil {
		Verbose("error: %v", err)
		return false
	}

	defer listener.Close()
	return true
}
