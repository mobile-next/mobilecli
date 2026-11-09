package machomap

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	fakeThread   = 1
	fakeLanding  = 0x1000
	fakeStuckPC  = 0x2000
	fakeReturned = 42
)

// fakeDebugserver answers the packets Target.Call sends. A borrowed thread that
// isStuck never returns on its own, as when another thread holds a lock it needs.
type fakeDebugserver struct {
	conn    net.Conn
	reader  *bufio.Reader
	isStuck bool
	pc      uint64
	packets []string

	refusesMemoryWrites bool
}

func (f *fakeDebugserver) reply(payload string) {
	_, _ = fmt.Fprintf(f.conn, "$%s#00", payload)
}

func (f *fakeDebugserver) readPacket() (string, error) {
	for {
		ch, err := f.reader.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == rspInterrupt {
			return string(rune(rspInterrupt)), nil
		}
		if ch != '$' {
			continue
		}
		body, err := f.reader.ReadString('#')
		if err != nil {
			return "", err
		}
		if _, err := f.reader.Discard(2); err != nil {
			return "", err
		}
		return strings.TrimSuffix(body, "#"), nil
	}
}

func registerValue(value uint64) string {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	return hex.EncodeToString(buf[:])
}

func (f *fakeDebugserver) serve() {
	soloResume := fmt.Sprintf("vCont;c:%x", fakeThread)
	readPC := fmt.Sprintf("p%x;thread:%x;", regPC, fakeThread)
	readX0 := fmt.Sprintf("p0;thread:%x;", fakeThread)
	for {
		packet, err := f.readPacket()
		if err != nil {
			return
		}
		f.packets = append(f.packets, packet)
		if f.refusesMemoryWrites && strings.HasPrefix(packet, "M") {
			f.reply("E01")
			continue
		}
		switch packet {
		case soloResume:
			if f.isStuck {
				continue // no stop reply until the interrupt arrives
			}
			f.pc = fakeLanding
			f.reply("T05")
		case string(rune(rspInterrupt)):
			f.pc = fakeStuckPC
			f.reply("T11")
		case "vCont;c":
			f.pc = fakeLanding
			f.reply("T05")
		case readPC:
			f.reply(registerValue(f.pc))
		case readX0:
			f.reply(registerValue(fakeReturned))
		default:
			f.reply("OK")
		}
	}
}

func borrowedThreadOn(t *testing.T, server *fakeDebugserver) *Target {
	t.Helper()
	client, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = serverConn.Close()
	})
	server.conn = serverConn
	server.reader = bufio.NewReader(serverConn)
	go server.serve()

	rsp := &RSP{w: client, r: bufio.NewReader(client), noAck: true}
	return &Target{c: rsp, thread: fakeThread, landing: fakeLanding, soloTimeout: 20 * time.Millisecond}
}

func TestCallKeepsOtherThreadsFrozenWhenItReturnsInTime(t *testing.T) {
	server := &fakeDebugserver{}
	target := borrowedThreadOn(t, server)

	returned, err := target.Call(0x3000)
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	if returned != fakeReturned {
		t.Errorf("returned %d, want %d", returned, fakeReturned)
	}
	if slices.Contains(server.packets, "vCont;c") {
		t.Error("other threads were resumed for a call that returned in time")
	}
}

func TestCallRunsAllThreadsWhenTheBorrowedThreadIsStuck(t *testing.T) {
	server := &fakeDebugserver{isStuck: true}
	target := borrowedThreadOn(t, server)

	returned, err := target.Call(0x3000)
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	if returned != fakeReturned {
		t.Errorf("returned %d, want %d", returned, fakeReturned)
	}
	if !slices.Contains(server.packets, "vCont;c") {
		t.Error("the stuck call was not finished with every thread running")
	}
}

func TestRestoreStillRestoresRegistersWhenTheStackWriteFails(t *testing.T) {
	server := &fakeDebugserver{refusesMemoryWrites: true}
	target := borrowedThreadOn(t, server)
	target.saved = "7"
	target.stackBackup = []byte{0}

	err := target.Restore()

	if err == nil {
		t.Error("the failed stack write was not reported")
	}
	restoreRegisters := fmt.Sprintf("QRestoreRegisterState:7;thread:%x;", fakeThread)
	if !slices.Contains(server.packets, restoreRegisters) {
		t.Error("registers were not restored after the stack write failed")
	}
	removeBreakpoint := fmt.Sprintf("z0,%x,4", fakeLanding)
	if !slices.Contains(server.packets, removeBreakpoint) {
		t.Error("the breakpoint was not removed after the stack write failed")
	}
}

func TestDecodingACutOffReplyDoesNotPanic(t *testing.T) {
	for _, wire := range []string{"ab}", "ab*", "*x"} {
		decodeRSP([]byte(wire))
	}
}
