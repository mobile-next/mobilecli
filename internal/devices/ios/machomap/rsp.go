package machomap

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// rspInterrupt is the out-of-band byte that stops a running process.
const rspInterrupt = 0x03

// RSP is a GDB Remote Serial Protocol client for the on-device debugserver. It
// fully decodes responses — run-length compression ('*') and byte escaping
// ('}') — which is required when we consume register and memory reads ourselves
// (rather than forwarding raw bytes to an external lldb). Mirrors the proven
// raw_attach.py prototype.
type RSP struct {
	w     io.Writer
	r     *bufio.Reader
	noAck bool
}

// NewRSP negotiates no-ack mode on an already-connected debugserver stream.
func NewRSP(rw io.ReadWriter) (*RSP, error) {
	c := &RSP{w: rw, r: bufio.NewReaderSize(rw, 1<<16)}
	if _, err := c.w.Write([]byte("+")); err != nil {
		return nil, err
	}
	resp, err := c.Request("QStartNoAckMode")
	if err != nil {
		return nil, err
	}
	if resp != "OK" {
		return nil, fmt.Errorf("no-ack handshake failed: %q", resp)
	}
	c.noAck = true
	return c, nil
}

// Request sends a packet and returns the decoded response payload, skipping
// asynchronous console-output ('O...') packets.
func (c *RSP) Request(pkt string) (string, error) {
	if err := c.send(pkt); err != nil {
		return "", err
	}
	for {
		resp, err := c.read()
		if err != nil {
			return "", err
		}
		if len(resp) > 1 && resp[0] == 'O' && resp != "OK" {
			continue // console output; ignore
		}
		return resp, nil
	}
}

// Interrupt stops the running process. It is safe to call while a Request is
// blocked waiting for a stop reply; that Request then returns the stop.
func (c *RSP) Interrupt() error {
	_, err := c.w.Write([]byte{rspInterrupt})
	return err
}

// Sync sends a packet whose answer is known and reads up to that answer, so a
// stray stop reply cannot be taken for the response to a later request.
func (c *RSP) Sync() error {
	if err := c.send("QThreadSuffixSupported"); err != nil {
		return err
	}
	for {
		resp, err := c.read()
		if err != nil {
			return err
		}
		if resp == "OK" {
			return nil
		}
	}
}

// OK sends a packet and requires the "OK" response.
func (c *RSP) OK(pkt string) error {
	resp, err := c.Request(pkt)
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("%.40s: %q", pkt, resp)
	}
	return nil
}

func (c *RSP) send(pkt string) error {
	var b strings.Builder
	b.WriteByte('$')
	sum := byte(0)
	for i := 0; i < len(pkt); i++ {
		ch := pkt[i]
		if ch == '#' || ch == '$' || ch == '*' || ch == '}' {
			b.WriteByte('}')
			b.WriteByte(ch ^ 0x20)
			sum += '}' + (ch ^ 0x20)
		} else {
			b.WriteByte(ch)
			sum += ch
		}
	}
	fmt.Fprintf(&b, "#%02x", sum)
	_, err := c.w.Write([]byte(b.String()))
	return err
}

// read reads one '$...#xx' packet and returns its decoded payload.
func (c *RSP) read() (string, error) {
	// skip to '$'
	for {
		ch, err := c.r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == '$' {
			break
		}
	}
	var wire []byte
	for {
		ch, err := c.r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == '#' {
			break
		}
		wire = append(wire, ch)
	}
	// consume the 2 checksum hex chars (not validated in no-ack mode, like raw_attach)
	if _, err := c.r.Discard(2); err != nil {
		return "", err
	}
	if !c.noAck {
		if _, err := c.w.Write([]byte("+")); err != nil {
			return "", err
		}
	}
	return decodeRSP(wire), nil
}

// decodeRSP expands '}' escaping and '*' run-length compression.
func decodeRSP(wire []byte) string {
	out := make([]byte, 0, len(wire))
	for i := 0; i < len(wire); i++ {
		ch := wire[i]
		switch ch {
		case '}':
			i++
			if i == len(wire) {
				return string(out)
			}
			out = append(out, wire[i]^0x20)
		case '*':
			i++
			if i == len(wire) || len(out) == 0 {
				return string(out)
			}
			repeat := int(wire[i]) - 29
			last := out[len(out)-1]
			for r := 0; r < repeat; r++ {
				out = append(out, last)
			}
		default:
			out = append(out, ch)
		}
	}
	return string(out)
}

// Memory reads size bytes at addr.
func (c *RSP) Memory(addr, size uint64) ([]byte, error) {
	resp, err := c.Request(fmt.Sprintf("m%x,%x", addr, size))
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(resp, "E") {
		return nil, fmt.Errorf("memory read @ %#x failed: %q", addr, resp)
	}
	data, err := hex.DecodeString(resp)
	if err != nil {
		return nil, fmt.Errorf("memory read @ %#x not hex: %w", addr, err)
	}
	if uint64(len(data)) != size {
		return nil, fmt.Errorf("short memory read @ %#x: got %d want %d", addr, len(data), size)
	}
	return data, nil
}

// WriteMem writes blob at addr in chunks (default 1024). The debugger can write
// pages the process itself cannot (e.g. r-x), the same W^X bypass it uses for
// breakpoints.
func (c *RSP) WriteMem(addr uint64, blob []byte, chunk int) error {
	if chunk <= 0 {
		chunk = 1024
	}
	for i := 0; i < len(blob); i += chunk {
		end := i + chunk
		if end > len(blob) {
			end = len(blob)
		}
		part := blob[i:end]
		if err := c.OK(fmt.Sprintf("M%x,%x:%s", addr+uint64(i), len(part), hex.EncodeToString(part))); err != nil {
			return err
		}
	}
	return nil
}

// readPtr reads a little-endian 64-bit pointer at addr.
func (c *RSP) readPtr(addr uint64) (uint64, error) {
	b, err := c.Memory(addr, 8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b), nil
}

// ReadReg reads a register by DWARF-ish number on a thread.
func (c *RSP) ReadReg(num int, thread uint64) (uint64, error) {
	resp, err := c.Request(fmt.Sprintf("p%x;thread:%x;", num, thread))
	if err != nil {
		return 0, err
	}
	if strings.HasPrefix(resp, "E") || resp == "" {
		return 0, fmt.Errorf("read reg %d: %q", num, resp)
	}
	raw, err := hex.DecodeString(resp)
	if err != nil {
		return 0, fmt.Errorf("read reg %d not hex: %w", num, err)
	}
	var buf [8]byte
	copy(buf[:], raw) // little-endian, may be shorter
	return binary.LittleEndian.Uint64(buf[:]), nil
}

// WriteReg sets a register by number on a thread.
func (c *RSP) WriteReg(num int, thread, value uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	return c.OK(fmt.Sprintf("P%x=%s;thread:%x;", num, hex.EncodeToString(buf[:]), thread))
}
