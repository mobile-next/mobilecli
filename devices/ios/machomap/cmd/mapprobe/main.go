// Command mapprobe is a throwaway on-device proof harness for the machomap
// package (Stage 1b). It dials the go-ios userspace tunnel forwarder exactly
// like the Python prototype (16-byte IPv6 + little-endian port), attaches to a
// pid, and maps a dylib with the host-files resolver so the Go result can be
// compared against inject_map.py. The production path (Stage 2) uses
// goios.ConnectTUNDevice instead of this forwarder and is wired into the device
// agent; this command is not shipped.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/mobile-next/mobilecli/devices/ios/machomap"
)

func main() {
	host := flag.String("host", "", "device tunnel IPv6 address")
	tunnelPort := flag.Int("tunnel-port", 0, "userspace tunnel forwarder local port")
	port := flag.Int("port", 0, "device debugproxy port")
	pid := flag.Int("pid", 0, "target process id")
	dylibPath := flag.String("dylib", "/tmp/agent-dev.dylib", "dylib to map")
	runInit := flag.Bool("init", false, "run the dylib initializer (starts the agent)")
	compare := flag.Bool("compare", false, "resolve helpers via host-files AND export-trie and compare, then exit")
	useTrie := flag.Bool("trie", false, "use the export-trie (host-independent) resolver for the map")
	symbolRoot := flag.String("symbol-root",
		"/Users/gilm/Library/Developer/Xcode/iOS DeviceSupport/iPhone12,1 26.5.2 (23F84)/Symbols/usr/lib",
		"Xcode device symbols root")
	flag.Parse()

	if *host == "" || *tunnelPort == 0 || *port == 0 || *pid == 0 {
		fmt.Fprintln(os.Stderr, "need --host --tunnel-port --port --pid")
		os.Exit(2)
	}

	dylib, err := os.ReadFile(*dylibPath)
	check(err)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", *tunnelPort), 8*time.Second)
	check(err)
	defer func() { _ = conn.Close() }()
	_ = conn.(*net.TCPConn).SetNoDelay(true)

	// userspace-tunnel forwarder header: 16-byte IPv6 + LE uint32 destination port
	ip := net.ParseIP(*host).To16()
	if ip == nil {
		fatal("bad --host IPv6 %q", *host)
	}
	hdr := make([]byte, 20)
	copy(hdr[:16], ip)
	binary.LittleEndian.PutUint32(hdr[16:], uint32(*port))
	_, err = conn.Write(hdr)
	check(err)

	c, err := machomap.NewRSP(conn)
	check(err)
	tConn := time.Now()

	if resp, err := c.Request("QSetDetachOnError:1"); err != nil || resp != "OK" {
		fatal("QSetDetachOnError: resp=%q err=%v", resp, err)
	}
	stop, err := c.Request(fmt.Sprintf("vAttach;%x", *pid))
	check(err)
	if len(stop) == 0 || stop[0] != 'T' {
		fatal("vAttach failed: %q", stop)
	}
	fmt.Printf("CONNECT %.3fs  vATTACH %.3fs\n", tConn.Sub(start).Seconds(), time.Since(tConn).Seconds())

	helperNames := []string{"mmap", "mprotect", "dlsym", "sel_registerName", "objc_readClassPair"}
	if *compare {
		hf, err := machomap.NewHostFilesResolver(c, *symbolRoot).Resolve(helperNames)
		check(err)
		et, err := machomap.NewExportTrieResolver(c).Resolve(helperNames)
		check(err)
		allMatch := true
		for _, n := range helperNames {
			ok := hf[n] == et[n]
			allMatch = allMatch && ok
			tag := "MATCH"
			if !ok {
				tag = "MISMATCH"
			}
			fmt.Printf("  %-20s host-files=%#x  export-trie=%#x  %s\n", n, hf[n], et[n], tag)
		}
		if allMatch {
			fmt.Println("ALL MATCH — export-trie resolver verified")
		} else {
			fmt.Println("MISMATCH — export-trie resolver is wrong")
		}
		_, _ = c.Request("D")
		return
	}

	var resolver machomap.Resolver = machomap.NewHostFilesResolver(c, *symbolRoot)
	if *useTrie {
		resolver = machomap.NewExportTrieResolver(c)
	}
	res, err := machomap.Map(c, dylib, resolver, machomap.Options{RunInit: *runInit})
	check(err)

	fmt.Printf("MAP base=%#x size=%#x\n", res.Base, res.ImageSize)
	fmt.Printf("FIXUPS: %d rebases, %d binds (%d imports)\n", res.Rebases, res.Binds, res.Imports)
	fmt.Printf("OBJC: %d selrefs, %d classes\n", res.Selrefs, res.Classes)
	if len(res.Unresolved) > 0 {
		fmt.Printf("UNRESOLVED (%d): %v\n", len(res.Unresolved), res.Unresolved)
	}
	if !*runInit {
		fmt.Println("(initializer NOT run; --init to start the agent)")
	}
	_, _ = c.Request("D")
	fmt.Printf("detached; total %.3fs\n", time.Since(start).Seconds())
}

func check(err error) {
	if err != nil {
		fatal("%v", err)
	}
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+f+"\n", a...)
	os.Exit(1)
}
