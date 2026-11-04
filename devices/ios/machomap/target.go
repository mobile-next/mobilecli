package machomap

import (
	"fmt"
	"strings"
)

// ARM64 register numbers as debugserver exposes them.
const (
	regLR = 30
	regSP = 31
	regPC = 32
)

// Target borrows a stopped thread in the attached process so we can call
// functions in it: save its registers, back up a stack window, and plant a
// breakpoint at a known return address. Restore() puts everything back.
type Target struct {
	c           *RSP
	thread      uint64
	saved       string
	returnPC    uint64
	sp          uint64
	stackLo     uint64
	stackBackup []byte
}

// NewTarget borrows the first enumerated thread (the main thread on the apps we
// target) and prepares it for calls.
func NewTarget(c *RSP) (*Target, error) {
	if err := c.OK("QThreadSuffixSupported"); err != nil {
		return nil, err
	}
	threads, err := c.Request("qfThreadInfo")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(threads, "m") {
		return nil, fmt.Errorf("cannot enumerate threads: %q", threads)
	}
	var thread uint64
	if _, err := fmt.Sscanf(strings.Split(threads[1:], ",")[0], "%x", &thread); err != nil {
		return nil, fmt.Errorf("parse thread id %q: %w", threads, err)
	}
	t := &Target{c: c, thread: thread}
	t.saved, err = c.Request(fmt.Sprintf("QSaveRegisterState;thread:%x;", thread))
	if err != nil {
		return nil, err
	}
	if !isDigits(t.saved) {
		return nil, fmt.Errorf("cannot save registers: %q", t.saved)
	}
	if t.returnPC, err = c.ReadReg(regPC, thread); err != nil {
		return nil, err
	}
	if t.sp, err = c.ReadReg(regSP, thread); err != nil {
		return nil, err
	}
	t.stackLo = (t.sp - 0x8000) &^ 15
	if t.stackBackup, err = c.Memory(t.stackLo, t.sp-t.stackLo); err != nil {
		return nil, err
	}
	if err := c.OK(fmt.Sprintf("Z0,%x,4", t.returnPC)); err != nil {
		return nil, err
	}
	return t, nil
}

// Call invokes fn(args...) on the borrowed thread and returns x0. LR points at
// the planted breakpoint, so the function returns into a known stop.
func (t *Target) Call(fn uint64, args ...uint64) (uint64, error) {
	for i, a := range args {
		if err := t.c.WriteReg(i, t.thread, a); err != nil {
			return 0, err
		}
	}
	if err := t.c.WriteReg(regLR, t.thread, t.returnPC); err != nil {
		return 0, err
	}
	if err := t.c.WriteReg(regSP, t.thread, t.sp-0x100); err != nil {
		return 0, err
	}
	if err := t.c.WriteReg(regPC, t.thread, fn); err != nil {
		return 0, err
	}
	resp, err := t.c.Request(fmt.Sprintf("vCont;c:%x", t.thread))
	if err != nil {
		return 0, err
	}
	pc, err := t.c.ReadReg(regPC, t.thread)
	if err != nil {
		return 0, err
	}
	if !strings.HasPrefix(resp, "T05") || pc != t.returnPC {
		return 0, fmt.Errorf("unexpected stop calling %#x: resp=%.60q pc=%#x", fn, resp, pc)
	}
	return t.c.ReadReg(0, t.thread)
}

// Restore undoes the borrow: stack window, registers, breakpoint.
func (t *Target) Restore() error {
	if err := t.c.WriteMem(t.stackLo, t.stackBackup, 1024); err != nil {
		return err
	}
	if err := t.c.OK(fmt.Sprintf("QRestoreRegisterState:%s;thread:%x;", t.saved, t.thread)); err != nil {
		return err
	}
	return t.c.OK(fmt.Sprintf("z0,%x,4", t.returnPC))
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
