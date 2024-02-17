package v0_test

import (
	"context"
	_ "embed"
	"net"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/gaukas/water"
	v0 "github.com/gaukas/water/transport/v0"
)

// This file is specifically designed to test to make sure everything will eventually be
// collected by the garbage collector. This is to ensure that there are no memory leaks
// on our end.

func TestConn_GC(t *testing.T) {
	var memStat runtime.MemStats
	var GCcount uint32

	// check if GC is enabled
	runtime.ReadMemStats(&memStat)
	GCcount = memStat.NumGC
	runtime.GC()
	runtime.Gosched()
	runtime.ReadMemStats(&memStat)
	if memStat.NumGC-GCcount == 0 {
		t.Skipf("Likely no GC enabled on this system, skipping test...")
	}

	tcpLis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpLis.Close() // skipcq: GO-S2307

	// Dial using water
	config := &water.Config{
		TransportModuleBin: wasmPlain,
	}
	dialer, err := v0.NewDialerWithContext(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}

	waterDialConn, err := dialer.DialContext(context.Background(), "tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// accept the connection
	tcpLisConn, err := tcpLis.Accept()
	if err != nil {
		t.Fatal(err)
	}

	var waterDialConnClosed atomic.Bool
	runtime.SetFinalizer(waterDialConn, func(c *v0.Conn) {
		waterDialConnClosed.Store(true)
	})

	var tcpLisConnClosed atomic.Bool
	runtime.SetFinalizer(tcpLisConn, func(c net.Conn) {
		tcpLisConnClosed.Store(true)
	})

	runtime.ReadMemStats(&memStat)
	GCcount = memStat.NumGC

	// close the connection
	waterDialConn.Close()
	tcpLisConn.Close()
	for {
		if waterDialConnClosed.Load() && tcpLisConnClosed.Load() {
			break
		}
		// if more than 10 GC cycles, then something is wrong
		runtime.ReadMemStats(&memStat)
		if memStat.NumGC-GCcount > 10 {
			if !waterDialConnClosed.Load() && tcpLisConnClosed.Load() {
				t.Fatal("water dialed Conn was not collected, but the peer was collected")
			} else {
				t.Skipf("Likely GC doesn't collect anything on this system, skipping test...")
			}
		}
		runtime.GC()
		runtime.Gosched()
	}
	runtime.ReadMemStats(&memStat)
	t.Logf("GC cycle taken: %d", memStat.NumGC-GCcount)

	// Listen using water
	lis, err := config.ListenContext(context.Background(), "tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close() // skipcq: GO-S2307

	// Dial using net
	tcpDialConn, err := net.Dial("tcp", lis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// accept the connection
	waterLisConn, err := lis.Accept()
	if err != nil {
		t.Fatal(err)
	}

	var tcpDialConnClosed atomic.Bool
	runtime.SetFinalizer(tcpDialConn, func(c net.Conn) {
		tcpDialConnClosed.Store(true)
	})

	var waterLisConnClosed atomic.Bool
	runtime.SetFinalizer(waterLisConn, func(c *v0.Conn) {
		waterLisConnClosed.Store(true)
	})

	runtime.ReadMemStats(&memStat)
	GCcount = memStat.NumGC

	// close the connection
	tcpDialConn.Close()
	waterLisConn.Close()

	for {
		if tcpDialConnClosed.Load() && waterLisConnClosed.Load() {
			break
		}
		// if more than 10 GC cycles, then something is wrong
		runtime.ReadMemStats(&memStat)
		if memStat.NumGC-GCcount > 10 {
			if !tcpDialConnClosed.Load() && waterLisConnClosed.Load() {
				t.Fatal("net dialed Conn was not collected, but the peer was collected")
			} else {
				t.Skipf("Likely GC doesn't collect anything on this system, skipping test...")
			}
		}
		runtime.GC()
		runtime.Gosched()
	}
	runtime.ReadMemStats(&memStat)
	t.Logf("GC cycle taken: %d", memStat.NumGC-GCcount)
}
