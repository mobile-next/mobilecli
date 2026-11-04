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
	bindtest := flag.Bool("bindtest", false, "test a loopback socket+bind in the target, then exit")
	servertest := flag.Bool("servertest", false, "map, then drive MobileServer alloc/init/bind via the runtime")
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

	if *servertest {
		// map first so MobileServer is registered, then drive it via the runtime
		if _, err := machomap.Map(c, dylib, machomap.NewExportTrieResolver(c), machomap.Options{RunInit: false}); err != nil {
			fatal("map: %v", err)
		}
		h, err := machomap.NewExportTrieResolver(c).Resolve([]string{"mmap", "objc_getClass", "objc_msgSend", "sel_registerName"})
		check(err)
		t, err := machomap.NewTarget(c)
		check(err)
		defer func() { _ = t.Restore() }()
		scratch, err := t.Call(h["mmap"], 0, 0x1000, 3, 0x1002, ^uint64(0), 0)
		check(err)
		cur := scratch
		put := func(s string) uint64 {
			b := append([]byte(s), 0)
			check(c.WriteMem(cur, b, 0))
			p := cur
			cur += uint64(len(b))
			return p
		}
		call := func(fn uint64, a ...uint64) uint64 { v, e := t.Call(fn, a...); check(e); return v }
		sel := func(s string) uint64 { return call(h["sel_registerName"], put(s)) }
		he, _ := machomap.NewExportTrieResolver(c).Resolve([]string{"__error"})
		errnoNow := func() uint64 {
			if ep, e := t.Call(he["__error"]); e == nil && ep != 0 {
				if b, e2 := c.Memory(ep, 4); e2 == nil {
					return uint64(b[0]) | uint64(b[1])<<8
				}
			}
			return 0
		}
		cls := call(h["objc_getClass"], put("MobileServer"))
		obj := call(h["objc_msgSend"], cls, sel("alloc"))
		obj = call(h["objc_msgSend"], obj, sel("initWithHandler:"), 0)
		bnd := call(h["objc_msgSend"], obj, sel("bind"))
		bErrno := errnoNow()
		port := call(h["objc_msgSend"], obj, sel("port"))
		fmt.Printf("MobileServer cls=%#x alloc/init obj=%#x  -bind=%d (errno=%d)  -port=%d\n",
			cls, obj, bnd&0xff, bErrno, port&0xffffffff)
		_, _ = c.Request("D")
		return
	}

	if *bindtest {
		h, err := machomap.NewExportTrieResolver(c).Resolve([]string{"mmap", "socket", "bind", "listen", "__error"})
		check(err)
		t, err := machomap.NewTarget(c)
		check(err)
		defer func() { _ = t.Restore() }()
		errnoNow := func() uint64 {
			if ep, e := t.Call(h["__error"]); e == nil && ep != 0 {
				if b, e2 := c.Memory(ep, 4); e2 == nil {
					return uint64(b[0]) | uint64(b[1])<<8
				}
			}
			return 0
		}
		scratch, err := t.Call(h["mmap"], 0, 0x1000, 3 /*RW*/, 0x1002 /*ANON|PRIVATE*/, ^uint64(0), 0)
		check(err)
		fd, err := t.Call(h["socket"], 2 /*AF_INET*/, 1 /*SOCK_STREAM*/, 0)
		check(err)
		// struct sockaddr_in: len=0, family=AF_INET(2), port=0, addr=127.0.0.1
		sa := []byte{0, 2, 0, 0, 0x7f, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
		check(c.WriteMem(scratch, sa, 0))
		bret, err := t.Call(h["bind"], fd, scratch, 16)
		check(err)
		bErrno := errnoNow()
		lret, err := t.Call(h["listen"], fd, 8)
		check(err)
		lErrno := errnoNow()
		fmt.Printf("socket fd=%d  bind ret=%d (errno=%d)  listen ret=%d (errno=%d)\n",
			int32(uint32(fd)), int32(uint32(bret)), bErrno, int32(uint32(lret)), lErrno)
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
	fmt.Printf("OBJC: %d selrefs, %d classes, %d category methods\n", res.Selrefs, res.Classes, res.Categories)
	if len(res.Unresolved) > 0 {
		fmt.Printf("UNRESOLVED (%d): %v\n", len(res.Unresolved), res.Unresolved)
	}
	if !*runInit {
		fmt.Println("(initializer NOT run; --init to start the agent)")
	} else {
		// boot check: the agent constructor binds a server and exports its port
		if m, err := machomap.ParseMacho(dylib); err == nil {
			if off, ok := m.Symbols["_mobilecli_get_port"]; ok {
				t, err := machomap.NewTarget(c)
				check(err)
				port, err := t.Call(res.Base + off)
				check(err)
				_ = t.Restore()
				fmt.Printf("mobilecli_get_port() = %d  (agent %s)\n", port&0xffffffff,
					map[bool]string{true: "RUNNING", false: "did not bind"}[port&0xffffffff != 0])
			}
		}
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
