package socket_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/gaukas/water/internal/socket"
)

func TestTCPConnPair(t *testing.T) {
	c1, c2, err := socket.TCPConnPair()
	if err != nil {
		if c1 == nil || c2 == nil {
			t.Fatal(err)
		} else { // likely due to Close() call errored
			t.Logf("socket.TCPConnPair returned non-fatal error: %v", err)
		}
	}

	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	// test c1 -> c2
	err = testIO(c1, c2, 1000, 1024, 0)
	if err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	// test c2 -> c1
	err = testIO(c2, c1, 1000, 1024, 0)
	if err != nil {
		t.Fatal(err)
	}
}
