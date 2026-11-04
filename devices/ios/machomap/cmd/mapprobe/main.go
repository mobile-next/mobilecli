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
		mres, err := machomap.Map(c, dylib, machomap.NewExportTrieResolver(c), machomap.Options{RunInit: false})
		if err != nil {
			fatal("map: %v", err)
		}
		// compile-time ivar-offset globals (what the compiled methods read) vs runtime
		if mm, e := machomap.ParseMacho(dylib); e == nil {
			ht, _ := machomap.NewExportTrieResolver(c).Resolve([]string{"mmap", "class_getInstanceVariable", "ivar_getOffset", "objc_getClass"})
			tt, _ := machomap.NewTarget(c)
			sc, _ := tt.Call(ht["mmap"], 0, 0x1000, 3, 0x1002, ^uint64(0), 0)
			cu := sc
			put := func(s string) uint64 {
				b := append([]byte(s), 0)
				_ = c.WriteMem(cu, b, 0)
				p := cu
				cu += uint64(len(b))
				return p
			}
			srvCls, _ := tt.Call(ht["objc_getClass"], put("MobileServer"))
			for _, iv := range []string{"_serverFd", "_port"} {
				off, ok := mm.Symbols["_OBJC_IVAR_$_MobileServer."+iv]
				if !ok {
					fmt.Printf("ivar %s: no symbol\n", iv)
					continue
				}
				gb, _ := c.Memory(mres.Base+off, 8)
				compiled := uint64(gb[0]) | uint64(gb[1])<<8 | uint64(gb[2])<<16 | uint64(gb[3])<<24
				ivar, _ := tt.Call(ht["class_getInstanceVariable"], srvCls, put(iv))
				rt, _ := tt.Call(ht["ivar_getOffset"], ivar)
				fmt.Printf("ivar %-10s compiled-global=%d runtime=%d %s\n", iv, compiled, rt&0xffffffff,
					map[bool]string{true: "MATCH", false: "MISMATCH"}[compiled == rt&0xffffffff])
			}
			_ = tt.Restore()
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
		clearErrno := func() {
			if ep, e := t.Call(he["__error"]); e == nil && ep != 0 {
				_ = c.WriteMem(ep, []byte{0, 0, 0, 0}, 0)
			}
		}
		// compare how the agent's imports bind (dlsym RTLD_DEFAULT) vs the specific
		// libsystem_kernel export (trie) for listen — a mismatch means RTLD_DEFAULT
		// picked the wrong image, which would break the agent's listen() call.
		hd, _ := machomap.NewExportTrieResolver(c).Resolve([]string{"dlsym", "listen"})
		listenDlsym := call(hd["dlsym"], ^uint64(0)-1 /*RTLD_DEFAULT*/, put("listen"))
		socketDlsym := call(hd["dlsym"], ^uint64(0)-1, put("socket"))
		fmt.Printf("listen: trie=%#x dlsym(RTLD_DEFAULT)=%#x  %s\n", hd["listen"], listenDlsym,
			map[bool]string{true: "MATCH", false: "MISMATCH"}[hd["listen"] == listenDlsym])
		fmt.Printf("socket dlsym=%#x\n", socketDlsym)
		cls := call(h["objc_getClass"], put("MobileServer"))
		obj := call(h["objc_msgSend"], cls, sel("alloc"))
		obj = call(h["objc_msgSend"], obj, sel("initWithHandler:"), 0)
		// sanity: a method that definitely returns YES, to validate BOOL reads
		resp := call(h["objc_msgSend"], obj, sel("respondsToSelector:"), sel("bind"))
		fmt.Printf("respondsToSelector(bind)=%d (want 1)\n", resp&0xff)
		// decompose on a fresh object: -bindPort alone
		o2 := call(h["objc_msgSend"], cls, sel("alloc"))
		o2 = call(h["objc_msgSend"], o2, sel("initWithHandler:"), 0)
		clearErrno()
		bp := call(h["objc_msgSend"], o2, sel("bindPort"))
		bpErr := errnoNow()
		p2 := call(h["objc_msgSend"], o2, sel("port"))
		fmt.Printf("-bindPort=%d (errno=%d) -port=%d\n", bp&0xff, bpErr, p2&0xffffffff)
		// read the agent socket's fd via its _serverFd ivar, then listen() on it directly
		hi, _ := machomap.NewExportTrieResolver(c).Resolve([]string{"class_getInstanceVariable", "ivar_getOffset", "listen"})
		ivar := call(hi["class_getInstanceVariable"], cls, put("_serverFd"))
		ivoff := call(hi["ivar_getOffset"], ivar)
		fdb, _ := c.Memory(o2+ivoff, 4)
		fd := uint64(fdb[0]) | uint64(fdb[1])<<8 | uint64(fdb[2])<<16 | uint64(fdb[3])<<24
		clearErrno()
		ld := call(hi["listen"], fd, 8)
		fmt.Printf("agent _serverFd=%d  direct listen(fd,8)=%d (errno=%d)\n",
			int32(uint32(fd)), int32(uint32(ld)), errnoNow())

		clearErrno()
		bnd := call(h["objc_msgSend"], obj, sel("bind"))
		bErrno := errnoNow()
		port := call(h["objc_msgSend"], obj, sel("port"))
		fmt.Printf("MobileServer cls=%#x alloc/init obj=%#x  -bind=%d (errno-after=%d)  -port=%d\n",
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
		// verify the agent booted: the constructor binds a control server (exported via
		// mobilecli_get_port) and installs the NSURLSession swizzle, after which
		// +[NSURLSessionConfiguration defaultSessionConfiguration]'s IMP points into our
		// mapped image.
		m, _ := machomap.ParseMacho(dylib)
		hv, err := machomap.NewExportTrieResolver(c).Resolve([]string{"mmap", "objc_getClass", "object_getClass", "sel_registerName", "class_getMethodImplementation"})
		check(err)
		t, err := machomap.NewTarget(c)
		check(err)
		defer func() { _ = t.Restore() }()
		scratch, err := t.Call(hv["mmap"], 0, 0x1000, 3, 0x1002, ^uint64(0), 0)
		check(err)
		cur := scratch
		put := func(s string) uint64 {
			b := append([]byte(s), 0)
			_ = c.WriteMem(cur, b, 0)
			p := cur
			cur += uint64(len(b))
			return p
		}
		call := func(fn uint64, a ...uint64) uint64 { v, e := t.Call(fn, a...); check(e); return v }

		if off, ok := m.Symbols["_mobilecli_get_port"]; ok {
			port := call(res.Base+off) & 0xffffffff
			fmt.Printf("control server: mobilecli_get_port() = %d  (%s)\n", port,
				map[bool]string{true: "UP", false: "not bound"}[port != 0])
		}
		meta := call(hv["object_getClass"], call(hv["objc_getClass"], put("NSURLSessionConfiguration")))
		imp := call(hv["class_getMethodImplementation"], meta, call(hv["sel_registerName"], put("defaultSessionConfiguration")))
		fmt.Printf("network swizzle: +[NSURLSessionConfiguration defaultSessionConfiguration] IMP=%#x  %s\n", imp,
			map[bool]string{true: "INSTALLED (points into mapped agent)", false: "NOT installed"}[imp >= res.Base && imp < res.Base+res.ImageSize])
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
