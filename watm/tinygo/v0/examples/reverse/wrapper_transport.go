package main

import (
	"io"

	v0 "github.com/refraction-networking/water/watm/tinygo/v0"
	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
)

var dupBuf []byte = make([]byte, 16384) // 16k buffer for reversing

// type guard: ReverseWrappingTransport must implement [v0.WrappingTransport].
var _ v0.WrappingTransport = (*ReverseWrappingTransport)(nil)

type ReverseWrappingTransport struct {
}

func (rwt *ReverseWrappingTransport) Wrap(conn v0net.Conn) (v0net.Conn, error) {
	return &ReverseConn{conn}, conn.SetNonBlock(true) // must set non-block, otherwise will block on read and lose fairness
}

type ReverseConn struct {
	v0net.Conn // embedded Conn
}

func (rc *ReverseConn) Read(b []byte) (n int, err error) {
	n, err = rc.Conn.Read(dupBuf)
	if err != nil {
		return 0, err
	}

	if n > len(b) {
		err = io.ErrShortBuffer
		n = len(b)
	}

	// reverse all bytes read successfully so far
	for i := 0; i < n; i++ {
		b[i] = dupBuf[n-i-1]
	}

	return n, err
}

func (rc *ReverseConn) Write(b []byte) (n int, err error) {
	// reverse the bytes to be written
	for i := 0; i < len(b); i++ {
		dupBuf[i] = b[len(b)-i-1]
	}

	return rc.Conn.Write(dupBuf)
}
