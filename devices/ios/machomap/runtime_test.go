package machomap

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

const (
	fakeClass          = 0xc1a55
	fakeSelector       = 0x5e1
	fakeMethod         = 0x3e7
	fakeImplementation = 0x1a9
	fakePort           = 49152
)

var fakeObjcHelpers = map[string]uint64{
	"objc_getClass":            0x4000,
	"sel_registerName":         0x4100,
	"class_getClassMethod":     0x4200,
	"method_getImplementation": 0x4300,
}

func wasWritten(server *fakeDebugserver, register int, value uint64) bool {
	return slices.Contains(server.packets, fmt.Sprintf("P%x=%s;thread:%x;", register, registerValue(value), fakeThread))
}

func wasCalled(server *fakeDebugserver, function uint64) bool {
	return wasWritten(server, regPC, function)
}

func TestClassMethodReturnsTheValueOfItsImplementation(t *testing.T) {
	server := &fakeDebugserver{returnValues: []uint64{fakeClass, fakeSelector, fakeMethod, fakeImplementation, fakePort}}
	target := borrowedThreadOn(t, server)

	value, registered, err := callClassMethod(target, fakeObjcHelpers, 1, 2)

	if err != nil || !registered || value != fakePort {
		t.Errorf("value = %d, registered = %v, err = %v; want %d, true and no error", value, registered, err, fakePort)
	}
	if !wasCalled(server, fakeImplementation) {
		t.Error("the implementation of the class method was not called")
	}
	if !wasWritten(server, 0, fakeClass) || !wasWritten(server, 1, fakeSelector) {
		t.Error("the class and selector were not passed as the first two arguments")
	}
}

func TestClassMethodOfAnUnregisteredClassReportsNoClass(t *testing.T) {
	server := &fakeDebugserver{returnValues: []uint64{0}}
	target := borrowedThreadOn(t, server)

	_, registered, err := callClassMethod(target, fakeObjcHelpers, 1, 2)

	if err != nil || registered {
		t.Errorf("registered = %v, err = %v; want false and no error", registered, err)
	}
}

func TestMissingClassMethodIsNeverCalled(t *testing.T) {
	server := &fakeDebugserver{returnValues: []uint64{fakeClass, fakeSelector, 0}}
	target := borrowedThreadOn(t, server)

	_, registered, err := callClassMethod(target, fakeObjcHelpers, 1, 2)

	if !registered || !errors.Is(err, ErrClassMethodNotFound) {
		t.Errorf("registered = %v, err = %v; want true and the missing method error", registered, err)
	}
	if wasCalled(server, fakeObjcHelpers["method_getImplementation"]) {
		t.Error("the missing method was still looked up")
	}
}
