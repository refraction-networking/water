package water

import (
	"bytes"
	"crypto/rand"
	"net"
	"os"
	"testing"
)

func TestRuntimeConnDialer(t *testing.T) {
	t.Run("testRuntimeConnDialerNoBG", testRuntimeConnDialerNoBG)
	if t.Failed() {
		t.Run("testRuntimeConnDialerNoBGGranularity", testRuntimeConnDialerNoBGGranularity)
	}
}

// this testcase directly calls (*RuntimeConnDialer).Dial() and
// fails the entire test suite if the call fails.
//
// It tests a RuntimeConnDialer that spawns no background goroutines.
func testRuntimeConnDialerNoBG(t *testing.T) {
	// listen on a local TCP port
	tcpListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer tcpListener.Close()

	// accept connections
	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() {
					// t.Logf("TCP server connection closing")
					c.Close()
				}()
				for {
					// read from conn, write back
					buf := make([]byte, 1024)
					n, err := c.Read(buf)
					if err != nil {
						return
					}

					t.Logf("TCP server reads: %x", buf[:n])

					_, err = c.Write(buf[:n])
					if err != nil {
						return
					}

					t.Logf("TCP server writes: %x", buf[:n])
				}
			}(conn)
		}
	}()

	rd := &RuntimeConnDialer{}
	rd.DebugMode()
	t.Logf("listening on %s", tcpListener.Addr().String())
	_, err = rd.Dial(tcpListener.Addr().String())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// load WASI binary from testdata
	wasi, err := os.ReadFile("testdata/wasi_template.wasi.wasm")
	if err != nil {
		t.Fatal(err)
	}
	rd.Config = &Config{
		WASI: wasi,
	}

	// dial again
	conn, err := rd.Dial(tcpListener.Addr().String())
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	defer conn.Close()

	// communication test: write 10 random messages and read back
	for i := 0; i < 10; i++ {
		var msg []byte = make([]byte, 64)
		n, err := rand.Read(msg)
		if err != nil {
			t.Fatal(err)
		}

		_, err = conn.Write(msg[:n])
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("TCP client writes: %x", msg[:n])

		buf := make([]byte, 1024)
		n, err = conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("TCP client reads: %x", buf[:n])

		if bytes.Equal(msg[:n], buf[:n]) {
			t.Log("TCP client: message echoed")
		} else {
			t.Fatal("TCP client: message not echoed")
		}
	}

	return
}

func testRuntimeConnDialerNoBGGranularity(t *testing.T) {
	// TODO: implement this for granular testing
	t.Skip("not implemented")
}
