package socket_test

import (
	"crypto/rand"
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/gaukas/water/socket"
)

func TestUnixConnPair(t *testing.T) {
	c1, c2, err := socket.UnixConnPair("")
	if err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	time.Sleep(1 * time.Second)

	// test c1 -> c2
	err = testIO(c1, c2, 10000, 1024, 0)
	if err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	time.Sleep(1 * time.Second)

	// test c2 -> c1
	err = testIO(c2, c1, 10000, 1024, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func testIO(wrConn, rdConn net.Conn, N int, sz int, sleep time.Duration) error {
	var sendMsg []byte = make([]byte, sz)
	rand.Read(sendMsg)

	var err error
	for i := 0; i < N; i++ {
		_, err = wrConn.Write(sendMsg)
		if err != nil {
			return fmt.Errorf("Write error: %w, cntr: %d, N: %d", err, i, N)
		}

		// receive data
		buf := make([]byte, 1024)
		_, err = rdConn.Read(buf)
		if err != nil {
			return fmt.Errorf("Read error: %w, cntr: %d, N: %d", err, i, N)
		}

		time.Sleep(sleep)
	}

	return nil
}
