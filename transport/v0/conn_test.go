// a //go:build unix && !windows && !exclude_v0

package v0_test

import (
	"crypto/rand"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gaukas/water"
	_ "github.com/gaukas/water/transport/v0"
)

var (
	// hexencoder_v0 []byte
	plain_v0 []byte
)

func BenchmarkConnV0(b *testing.B) {
	// read file into plain_v0
	var err error
	plain_v0, err = os.ReadFile("../../testdata/plain_v0.wasm")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("PlainV0-Dialer", benchmarkPlainV0Dialer)
	b.Run("PlainV0-Listener", benchmarkPlainV0Listener)
	b.Run("RefTCP", benchmarkReferenceTCP)
}

func benchmarkPlainV0Dialer(b *testing.B) {
	// create random TCP listener listening on localhost
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		b.Fatal(err)
	}
	defer tcpLis.Close()

	// b.Logf("listener: %s", tcpLis.Addr())

	// goroutine to accept incoming connections
	var lisConn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lisConn, goroutineErr = tcpLis.Accept()
	}()

	// Dial
	config := &water.Config{
		TMBin: plain_v0,
	}
	// config.WASIConfig().InheritStdout()
	dialer, err := water.NewDialer(config)
	if err != nil {
		b.Fatal(err)
	}

	rConn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer rConn.Close()

	// b.Logf("dialer: %s, listener: %s", rConn.LocalAddr(), rConn.RemoteAddr())

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 1024)
	_, err = rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	// b.Logf("sendMsg: %s", sendMsg)

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	b.SetBytes(1024)
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		// b.Logf("writing...")
		_, err = rConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		// b.Logf("reading...")
		buf := make([]byte, 1024+128)
		_, err = lisConn.Read(buf)
		if err != nil {
			b.Logf("Read error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.Logf("avg bandwidth: %f MB/s (N=%d)", float64(b.N*1024)/time.Since(start).Seconds()/1024/1024, b.N)
}

func benchmarkPlainV0Listener(b *testing.B) {
	// prepare for listener
	config := &water.Config{
		TMBin: plain_v0,
	}

	lis, err := config.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatal(err)
	}

	// goroutine to dial listener
	var dialConn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		dialConn, goroutineErr = net.Dial("tcp", lis.Addr().String())
	}()

	// Accept
	rConn, err := lis.Accept()
	if err != nil {
		b.Fatal(err)
	}

	// wait for dialer to dial
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 512)
	_, err = rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	b.SetBytes(1024) // we will send 512-byte data and 128-byte will be transmitted on wire due to hex encoding
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		_, err = rConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		// receive data
		buf := make([]byte, 1024)
		_, err = dialConn.Read(buf)
		if err != nil {
			b.Logf("Read error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}
	}
	b.StopTimer()
	b.Logf("avg bandwidth: %f MB/s (N=%d)", float64(b.N*1024)/time.Since(start).Seconds()/1024/1024, b.N)

	if err = rConn.Close(); err != nil {
		b.Fatal(err)
	}
}

func benchmarkReferenceTCP(b *testing.B) {
	// create random TCP listener listening on localhost
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		b.Fatal(err)
	}
	defer tcpLis.Close()

	// goroutine to accept incoming connections
	var lisConn net.Conn
	var goroutineErr error
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		lisConn, goroutineErr = tcpLis.Accept()
	}()

	nConn, err := net.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 1024)
	_, err = rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	b.SetBytes(1024)
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		_, err = nConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		// receive data
		buf := make([]byte, 1024)
		_, err = lisConn.Read(buf)
		if err != nil {
			b.Logf("Read error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		// time.Sleep(10 * time.Microsecond)
	}
	b.StopTimer()
	b.Logf("avg bandwidth: %f MB/s (N=%d)", float64(b.N*1024)/time.Since(start).Seconds()/1024/1024, b.N)

	if err = nConn.Close(); err != nil {
		b.Fatal(err)
	}
}
