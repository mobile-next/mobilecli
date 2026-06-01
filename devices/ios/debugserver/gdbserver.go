// Package debugserver implements the wire-level GDB Remote Serial Protocol
// client used to talk to the on-device debugserver (via the CoreDevice debug
// proxy).
//
// Vendored from github.com/danielpaulus/go-ios/ios/debugserver
// (only the GDBServer type from lockdownconn.go, which is dependency-free).
// The sole change from upstream is raising the bufio.Scanner buffer limit:
// debugserver answers jGetLoadedDynamicLibrariesInfos with the full loaded-image
// list as a single packet that easily exceeds bufio's default 64 KB token size,
// which made LLDB load zero images (no symbols → expr injection failed).
// Original code is MIT licensed — see the LICENSE file in this directory.
package debugserver

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
)

var ErrInvalidGDBServerPayload = errors.New("invalid payload")

// maxPacketSize bounds a single GDB-RSP packet we will read from the device.
// The image-list response (jGetLoadedDynamicLibrariesInfos) for a real app is
// hundreds of KB to a few MB, so we allow up to 64 MB.
const maxPacketSize = 64 * 1024 * 1024

type GDBServer struct {
	rw      io.ReadWriter
	scanner *bufio.Scanner
}

// Implements wire level GDBServer protocol
func NewGDBServer(rw io.ReadWriter) *GDBServer {
	scanner := bufio.NewScanner(rw)
	// Raise the token limit; the default (bufio.MaxScanTokenSize, 64 KB) is far
	// too small for the image-list packet and yields "token too long".
	scanner.Buffer(make([]byte, 0, 64*1024), maxPacketSize)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		const lenPacketSuffix = 3 // len("#00")

		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		start := bytes.IndexRune(data, '$')
		end := bytes.IndexRune(data, '#')
		// Need more data
		if start < 0 || end < 0 || len(data) < end+lenPacketSuffix {
			return 0, nil, nil
		}
		// Invalid data
		if end < start {
			return 0, nil, ErrInvalidGDBServerPayload
		}

		// Strip the $ prefix before returning
		return end + lenPacketSuffix, data[start+1 : end], nil
	})

	return &GDBServer{
		rw:      rw,
		scanner: scanner,
	}
}

func (*GDBServer) chksum(pck string) string {
	sum := 0
	for _, b := range pck {
		sum += int(b)
	}
	return hex.EncodeToString([]byte{byte(sum % 256)})
}

func (g *GDBServer) formatPacket(pck string) string {
	return "+$" + pck + "#" + g.chksum(pck)
}

func (g *GDBServer) Recv() (string, error) {
	if g.scanner.Scan() == false {
		return "", g.scanner.Err()
	}
	return g.scanner.Text(), nil
}

func (g *GDBServer) Send(req string) error {
	pck := g.formatPacket(req)
	if _, err := g.rw.Write([]byte(pck)); err != nil {
		return err
	}
	return nil
}

func (g *GDBServer) Request(req string) (string, error) {
	if err := g.Send(req); err != nil {
		return "", err
	}
	return g.Recv()
}
