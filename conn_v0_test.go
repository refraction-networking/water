//go:build unix && !windows && !nov0

package water_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gaukas/water"
)

var hexencoder_v0 []byte
var plain_v0 []byte

func TestConnV0(t *testing.T) {
	// read file into hexencoder_v0
	var err error
	hexencoder_v0, err = os.ReadFile("./testdata/hexencoder_v0.wasm")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("DialerV0", testDialerV0)
	t.Run("ListenerV0", testListenerV0)
}

func testDialerV0(t *testing.T) {
	// t.Parallel()

	// create random TCP listener listening on localhost
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
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

	// Dial
	dialer := &water.Dialer{
		Config: &water.Config{
			WATMBin: hexencoder_v0,
			WATMConfig: water.WATMConfig{
				FilePath: "./testdata/hexencoder_v0.dialer.json",
			},
		},
	}
	dialer.Config.WASIConfig().InheritStdout()

	rConn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		t.Fatal(goroutineErr)
	}

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("hello"), []byte("world")); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("i'm dialer"), []byte("hello dialer")); err != nil {
		t.Fatal(err)
	}

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("who are you?"), []byte("I'm listener")); err != nil {
		t.Fatal(err)
	}
}

func testListenerV0(t *testing.T) {
	// t.Parallel()

	// prepare for listener
	config := &water.Config{
		WATMBin: hexencoder_v0,
		WATMConfig: water.WATMConfig{
			FilePath: "./testdata/hexencoder_v0.listener.json",
		},
		// WASIConfigFactory: wasm.NewWasiConfigFactory(),
	}
	config.WASIConfig().InheritStdout()

	lis, err := config.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	defer rConn.Close()

	// wait for dialer to dial
	wg.Wait()
	if goroutineErr != nil {
		t.Fatal(goroutineErr)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("hello"), []byte("world")); err != nil {
		t.Error(err)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("i'm listener"), []byte("hello listener")); err != nil {
		t.Error(err)
	}

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("who are you?"), []byte("I'm dialer")); err != nil {
		t.Error(err)
	}
}

func testUppercaseHexencoderConn(encoderConn, plainConn net.Conn, dMsg, lMsg []byte) error {
	// dConn -> lConn
	_, err := encoderConn.Write(dMsg)
	if err != nil {
		return err
	}

	// receive data
	buf := make([]byte, 1024)
	n, err := plainConn.Read(buf)
	if err != nil {
		return err
	}

	// decode hex
	var decoded []byte = make([]byte, 1024)
	n, err = hex.Decode(decoded, buf[:n])
	if err != nil {
		return err
	}

	// compare received bytes with expected bytes
	if string(decoded[:n]) != string(dMsg) {
		return fmt.Errorf("expected: %s, got: %s", dMsg, decoded[:n])
	}

	// encode hex
	var encoded []byte = make([]byte, 1024)
	n = hex.Encode(encoded, lMsg)

	// lConn -> dConn
	_, err = plainConn.Write(encoded[:n])
	if err != nil {
		return err
	}

	// receive data
	n, err = encoderConn.Read(buf)
	if err != nil {
		return err
	}

	// compare received bytes with expected bytes
	var upperLMsg []byte = make([]byte, len(lMsg))
	for i, b := range lMsg {
		if b >= 'a' && b <= 'z' { // to uppercase
			upperLMsg[i] = b - 32
		} else {
			upperLMsg[i] = b
		}
	}

	if string(buf[:n]) != string(upperLMsg) {
		return fmt.Errorf("expected: %s, got: %s", upperLMsg, decoded[:n])
	}

	return nil
}

func testLowercaseHexencoderConn(encoderConn, plainConn net.Conn, dMsg, lMsg []byte) error {
	// dConn -> lConn
	_, err := encoderConn.Write(dMsg)
	if err != nil {
		return err
	}

	// receive data
	buf := make([]byte, 1024)
	n, err := plainConn.Read(buf)
	if err != nil {
		return err
	}

	// decode hex
	var decoded []byte = make([]byte, 1024)
	n, err = hex.Decode(decoded, buf[:n])
	if err != nil {
		return err
	}

	// compare received bytes with expected bytes
	if string(decoded[:n]) != string(dMsg) {
		return fmt.Errorf("expected: %s, got: %s", dMsg, decoded[:n])
	}

	// encode hex
	var encoded []byte = make([]byte, 1024)
	n = hex.Encode(encoded, lMsg)

	// lConn -> dConn
	_, err = plainConn.Write(encoded[:n])
	if err != nil {
		return err
	}

	// receive data
	n, err = encoderConn.Read(buf)
	if err != nil {
		return err
	}

	// compare received bytes with expected bytes
	var upperLMsg []byte = make([]byte, len(lMsg))
	for i, b := range lMsg {
		if b >= 'A' && b <= 'Z' { // to lowercase
			upperLMsg[i] = b + 32
		} else {
			upperLMsg[i] = b
		}
	}

	if string(buf[:n]) != string(upperLMsg) {
		return fmt.Errorf("expected: %s, got: %s", upperLMsg, decoded[:n])
	}

	return nil
}

func BenchmarkConnV0(b *testing.B) {
	// read file into plain_v0
	var err error
	plain_v0, err = os.ReadFile("./testdata/plain_v0.wasm")
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
	dialer := &water.Dialer{
		Config: &water.Config{
			WATMBin: plain_v0,
		},
	}

	rConn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer rConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 1024)
	rand.Read(sendMsg)

	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	b.SetBytes(1024)
	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		_, err = rConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

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
		WATMBin: plain_v0,
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
	defer rConn.Close()

	// wait for dialer to dial
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 512)
	rand.Read(sendMsg)

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
	defer nConn.Close()

	// wait for listener to accept connection
	wg.Wait()
	if goroutineErr != nil {
		b.Fatal(goroutineErr)
	}

	var sendMsg []byte = make([]byte, 1024)
	rand.Read(sendMsg)

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
}
