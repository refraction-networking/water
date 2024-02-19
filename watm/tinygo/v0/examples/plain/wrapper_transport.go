package main

import (
	v0 "github.com/refraction-networking/water/watm/tinygo/v0"
	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
)

// type guard: PlainWrappingTransport must implement [v0.WrappingTransport].
var _ v0.WrappingTransport = (*PlainWrappingTransport)(nil)

type PlainWrappingTransport struct {
}

func (rwt *PlainWrappingTransport) Wrap(conn v0net.Conn) (v0net.Conn, error) {
	return &PlainConn{conn}, conn.SetNonBlock(true) // must set non-block, otherwise will block on read and lose fairness
}

// PlainConn simply passes through the underlying Conn.
type PlainConn struct {
	v0net.Conn // embedded Conn
}
