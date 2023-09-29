//go:build unix && !windows

package water_test

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gaukas/water"
)

var hexencoder_v0 []byte

func TestRuntimeConnV0(t *testing.T) {
	// read file into hexencoder_v0
	var err error
	hexencoder_v0, err = os.ReadFile("./testdata/hexencoder_v0.wasm")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Dialer", testDialerV0)
	t.Run("Listener", testListenerV0)
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
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		lisConn, err = tcpLis.Accept()
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Dial
	dialer := &water.Dialer{
		Config: &water.Config{
			WABin: hexencoder_v0,
			WAConfig: water.WAConfig{
				FilePath: "./testdata/hexencoder_v0.dialer.json",
			},
			WASIConfigFactory: water.NewWasiConfigFactory(),
		},
	}
	dialer.Config.WASIConfigFactory.InheritStdout()

	rConn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rConn.Close()

	// wait for listener to accept connection
	wg.Wait()

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("hello"), []byte("world")); err != nil {
		t.Fatal(err)
	}

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("i'm dialer"), []byte("hello dialer")); err != nil {
		t.Fatal(err)
	}

	if err = testUppercaseHexencoderConn(rConn, lisConn, []byte("who are you?"), []byte("I'm listener")); err != nil {
		t.Fatal(err)
	}
}

func testListenerV0(t *testing.T) {
	// t.Parallel()

	// prepare for listener
	config := &water.Config{
		WABin: hexencoder_v0,
		WAConfig: water.WAConfig{
			FilePath: "./testdata/hexencoder_v0.listener.json",
		},
		WASIConfigFactory: water.NewWasiConfigFactory(),
	}
	config.WASIConfigFactory.InheritStdout()

	lis, err := water.ListenConfig("tcp", "localhost:0", config)
	if err != nil {
		t.Fatal(err)
	}

	// goroutine to dial listener
	var dialConn net.Conn
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		dialConn, err = net.Dial("tcp", lis.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Accept
	rConn, err := lis.Accept()
	if err != nil {
		t.Fatal(err)
	}

	// wait for dialer to dial
	wg.Wait()

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("hello"), []byte("world")); err != nil {
		t.Fatal(err)
	}

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("i'm listener"), []byte("hello listener")); err != nil {
		t.Fatal(err)
	}

	if err = testLowercaseHexencoderConn(rConn, dialConn, []byte("who are you?"), []byte("I'm dialer")); err != nil {
		t.Fatal(err)
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

func BenchmarkRuntimeConnV0(b *testing.B) {
	// read file into hexencoder_v0
	var err error
	hexencoder_v0, err = os.ReadFile("./testdata/hexencoder_v0.wasm")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("Dialer", benchmarkDialerV0)
	// b.Run("Listener", benchmarkListenerV0)
}

func benchmarkDialerV0(b *testing.B) {
	// create random TCP listener listening on localhost
	tcpLis, err := net.ListenTCP("tcp", nil)
	if err != nil {
		b.Fatal(err)
	}
	defer tcpLis.Close()

	// goroutine to accept incoming connections
	var lisConn net.Conn
	var wg *sync.WaitGroup = new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		lisConn, err = tcpLis.Accept()
		if err != nil {
			b.Fatal(err)
		}
	}()

	// Dial
	dialer := &water.Dialer{
		Config: &water.Config{
			WABin: hexencoder_v0,
			WAConfig: water.WAConfig{
				FilePath: "./testdata/hexencoder_v0.dialer.json",
			},
			WASIConfigFactory: water.NewWasiConfigFactory(),
		},
	}
	dialer.Config.WASIConfigFactory.InheritStdout()

	rConn, err := dialer.Dial("tcp", tcpLis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	defer rConn.Close()

	// wait for listener to accept connection
	wg.Wait()

	b.ResetTimer()
	b.SetBytes(1024) // we will send 512-byte data and 128-byte will be transmitted on wire due to hex encoding
	var sendMsg []byte = make([]byte, 512)
	rand.Read(sendMsg)
	for i := 0; i < b.N; i++ {
		_, err = rConn.Write(sendMsg)
		if err != nil {
			b.Logf("cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		// receive data
		buf := make([]byte, 1024)
		_, err = lisConn.Read(buf)
		if err != nil {
			b.Logf("cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		time.Sleep(10 * time.Microsecond)
	}
}
