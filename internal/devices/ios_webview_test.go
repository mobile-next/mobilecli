package devices

import (
	"strings"
	"testing"
)

const lldbOutputWhenAgentLoads = `(lldb) expr (void*)dlopen("/tmp/mobilecli-agent-1.dylib", 2)
(void *) $0 = 0x0000000102a0c4e0
(lldb) expr (char*)dlerror()
(char *) $1 = 0x0000000000000000
(lldb) expr (int)mobilecli_get_port()
(int) $2 = 27042
`

const lldbOutputWhenAppIsX8664 = `(lldb) expr (void*)dlopen("/tmp/mobilecli-agent-1.dylib", 2)
(void *) $0 = 0x0000000000000000
(lldb) expr (char*)dlerror()
(char *) $1 = 0x0000600000c04000 "dlopen(/tmp/mobilecli-agent-1.dylib, 0x0002): tried: '/tmp/mobilecli-agent-1.dylib' (mach-o file, but is an incompatible architecture (have 'arm64', need 'x86_64'))"
(lldb) expr (int)mobilecli_get_port()
error: use of undeclared identifier 'mobilecli_get_port'
`

func TestReadsAgentPortWhenAgentLoads(t *testing.T) {
	port, err := portFromInjectOutput(lldbOutputWhenAgentLoads)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port != 27042 {
		t.Fatalf("port = %d, want 27042", port)
	}
}

func TestReportsDlerrorWhenAgentCannotLoad(t *testing.T) {
	_, err := portFromInjectOutput(lldbOutputWhenAppIsX8664)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "could not load the agent into the app:") || !strings.Contains(err.Error(), "incompatible architecture") {
		t.Fatalf("error does not name the cause: %v", err)
	}
}

func TestReportsLLDBOutputWhenNoPortAndNoDlerror(t *testing.T) {
	_, err := portFromInjectOutput("(lldb) detach\n")
	if err == nil || !strings.Contains(err.Error(), "could not parse port") {
		t.Fatalf("unexpected error: %v", err)
	}
}
