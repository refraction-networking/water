//go:build !wasip1 && !wasi

package net_test

import (
	"bytes"
	"errors"
	"io"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"

	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
)

func tcpConnPair() (*net.TCPConn, *net.TCPConn, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil, err
	}

	dialConn, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		return nil, nil, err
	}

	acceptConn, err := l.Accept()
	if err != nil {
		return nil, nil, err
	}

	return dialConn.(*net.TCPConn), acceptConn.(*net.TCPConn), nil
}

func TestTCPConn_Read(t *testing.T) {
	conn1, conn2, err := tcpConnPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	defer conn2.Close()

	var msgWr = []byte("hello")
	var bufRd = make([]byte, 16)

	if _, err := conn1.Write(msgWr); err != nil {
		t.Fatal(err)
	}

	// rebuild conn2 as a *TCPConn
	var tcpConn2 *v0net.TCPConn

	// expose conn2's fd
	rawConn2, err := conn2.SyscallConn()
	if err != nil {
		t.Fatal(err)
	} else if err := rawConn2.Control(func(fd uintptr) {
		tcpConn2 = v0net.RebuildTCPConn(int32(fd))
	}); err != nil {
		t.Fatal(err)
	}

	n, err := tcpConn2.Read(bufRd)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(bufRd[:n], msgWr) {
		t.Fatalf("read: expected %s, got %s", msgWr, bufRd[:n])
	}

	// close the peer connection and read again
	conn1.Close()
	if _, err := tcpConn2.Read(bufRd); err == nil {
		t.Fatal("read after peer-close: expected error, got nil")
	}

	runtime.KeepAlive(conn1)
	runtime.KeepAlive(conn2)
}

func TestTCPConn_Write(t *testing.T) {
	conn1, conn2, err := tcpConnPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	defer conn2.Close()

	var msgWr = []byte("hello")
	var bufRd = make([]byte, 16)

	// rebuild conn1 as a *TCPConn
	var tcpConn1 *v0net.TCPConn

	// expose conn1's fd
	rawConn1, err := conn1.SyscallConn()
	if err != nil {
		t.Fatal(err)
	} else if err := rawConn1.Control(func(fd uintptr) {
		tcpConn1 = v0net.RebuildTCPConn(int32(fd))
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := tcpConn1.Write(msgWr); err != nil {
		t.Fatal(err)
	}

	n, err := conn2.Read(bufRd)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(bufRd[:n], msgWr) {
		t.Fatalf("expected %s, got %s", msgWr, bufRd[:n])
	}

	runtime.KeepAlive(conn1)
	runtime.KeepAlive(conn2)
}

func TestTCPConn_Close(t *testing.T) {
	conn1, conn2, err := tcpConnPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	defer conn2.Close()

	// rebuild conn1 as a *TCPConn
	var tcpConn1 *v0net.TCPConn

	// expose conn1's fd
	rawConn1, err := conn1.SyscallConn()
	if err != nil {
		t.Fatal(err)
	} else if err := rawConn1.Control(func(fd uintptr) {
		tcpConn1 = v0net.RebuildTCPConn(int32(fd))
	}); err != nil {
		t.Fatal(err)
	}

	if err := tcpConn1.Close(); err != nil {
		t.Fatal(err)
	}

	// At this point, conn1 (and the rebuilt tcpConn1) should not be writable
	if _, err := conn1.Write([]byte("hello")); err == nil {
		t.Fatal("expected error, got nil")
	} else if _, err := tcpConn1.Write([]byte("hello")); err == nil {
		t.Fatal("expected error, got nil")
	}

	// Similarly, all tcpConn1, conn1 and conn2 should not be readable
	var bufRd = make([]byte, 16)
	if _, err := tcpConn1.Read(bufRd); err == nil {
		t.Fatal("expected error, got nil")
	} else if _, err := conn1.Read(bufRd); err == nil {
		t.Fatal("expected error, got nil")
	} else if _, err := conn2.Read(bufRd); !errors.Is(err, io.EOF) && !errors.Is(err, syscall.ECONNRESET) {
		t.Fatalf("expected io.EOF or syscall.ECONNRESET, got %v", err)
	}

	runtime.KeepAlive(conn1)
	runtime.KeepAlive(conn2)
}

func TestTCPConn_SetNonBlock(t *testing.T) {
	conn1, conn2, err := tcpConnPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	defer conn2.Close()

	// rebuild conn1 as a *TCPConn
	var tcpConn1 *v0net.TCPConn

	// expose conn1's fd
	rawConn1, err := conn1.SyscallConn()
	if err != nil {
		t.Fatal(err)
	} else if err := rawConn1.Control(func(fd uintptr) {
		tcpConn1 = v0net.RebuildTCPConn(int32(fd))
	}); err != nil {
		t.Fatal(err)
	}

	if err := tcpConn1.SetNonBlock(true); err != nil {
		t.Fatal(err)
	}

	// since conn2 has not been written to, tcpConn1 should not be readable.
	if _, err := tcpConn1.Read(make([]byte, 16)); !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("expected %s, got %s", syscall.EAGAIN, err)
	}

	// write to conn2
	if _, err := conn2.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Microsecond) // wait for the packet to get through

	// now tcpConn1 should be readable
	if _, err := tcpConn1.Read(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}

	runtime.KeepAlive(conn1)
	runtime.KeepAlive(conn2)
}

func TestTCPConn_SetReadDeadline(t *testing.T) {
	conn1, conn2, err := tcpConnPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn1.Close()
	defer conn2.Close()

	// rebuild conn1 as a *TCPConn
	var tcpConn1 *v0net.TCPConn

	// expose conn1's fd
	rawConn1, err := conn1.SyscallConn()
	if err != nil {
		t.Fatal(err)
	} else if err := rawConn1.Control(func(fd uintptr) {
		tcpConn1 = v0net.RebuildTCPConn(int32(fd))
	}); err != nil {
		t.Fatal(err)
	}

	// set read deadline to 10ms later
	timeStart := time.Now()
	if err := tcpConn1.SetReadDeadline(timeStart.Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	// since conn2 has not been written to, tcpConn1 should not be readable.
	if _, err := tcpConn1.Read(make([]byte, 16)); !errors.Is(err, syscall.EAGAIN) {
		t.Fatalf("expected %s, got %s", syscall.EAGAIN, err)
	}
	if time.Since(timeStart) < 10*time.Millisecond {
		t.Fatalf("expected read to block for at least 10ms, but it only blocked for %s", time.Since(timeStart))
	}

	// write to conn2
	if _, err := conn2.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	// now tcpConn1 should be readable
	if err := tcpConn1.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	if _, err := tcpConn1.Read(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}

	runtime.KeepAlive(conn1)
	runtime.KeepAlive(conn2)
}
