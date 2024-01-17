package v0_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	_ "embed"
)

var (
	//go:embed testdata/plain.wasm
	wasmPlain []byte

	//go:embed testdata/reverse.wasm
	wasmReverse []byte
)

func benchmarkUnidirectionalStream(b *testing.B, wrConn, rdConn net.Conn) {
	var sendMsg []byte = make([]byte, 1024)
	_, err := rand.Read(sendMsg)
	if err != nil {
		b.Fatalf("rand.Read error: %s", err)
	}

	// setup a goroutine to read from the peerConn
	var wg2 *sync.WaitGroup = new(sync.WaitGroup)
	var peerRecvErr error
	wg2.Add(1)
	go func() {
		defer wg2.Done()
		recvBytes := 0
		var n int
		recvbuf := make([]byte, 1024+1) //
		for recvBytes < b.N*1024 {
			n, peerRecvErr = rdConn.Read(recvbuf)
			recvBytes += n
			if peerRecvErr != nil {
				return
			}
		}
	}()

	runtime.GC()
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Microsecond)

	b.SetBytes(1024)
	b.StartTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = wrConn.Write(sendMsg)
		if err != nil {
			b.Logf("Write error, cntr: %d, N: %d", i, b.N)
			b.Fatal(err)
		}

		_, err := rand.Read(sendMsg)
		if err != nil {
			b.Fatalf("rand.Read error: %s", err)
		}
	}
	wg2.Wait()
	b.StopTimer()

	if peerRecvErr != nil {
		b.Fatal(peerRecvErr)
	}
}

func sanityCheckConn(wrConn, rdConn net.Conn, writeMsg, expectRead []byte) error {
	_, err := wrConn.Write(writeMsg)
	if err != nil {
		return err
	}

	recvbuf := make([]byte, len(expectRead)+1)
	n, err := rdConn.Read(recvbuf)
	if err != nil {
		return err
	}

	if n != len(expectRead) {
		return errors.New("read length mismatch")
	}

	if !bytes.Equal(recvbuf[:n], expectRead) {
		return errors.New("read content mismatch")
	}

	return nil
}
