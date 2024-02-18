package main

import (
	tls "github.com/refraction-networking/utls"
	v0 "github.com/refraction-networking/water/watm/tinygo/v0"
	v0net "github.com/refraction-networking/water/watm/tinygo/v0/net"
)

// type guard: ReverseWrappingTransport must implement [v0.WrappingTransport].
var _ v0.WrappingTransport = (*UTLSClientWrappingTransport)(nil)

type UTLSClientWrappingTransport struct {
}

func (uwt *UTLSClientWrappingTransport) Wrap(conn v0net.Conn) (v0net.Conn, error) {
	tlsConn := tls.UClient(conn, &tls.Config{InsecureSkipVerify: true}, tls.HelloChrome_Auto)
	if err := tlsConn.Handshake(); err != nil {
		return nil, err
	}

	if err := conn.SetNonBlock(true); err != nil {
		return nil, err
	}

	return &UTLSConn{
		Conn:    conn,
		tlsConn: tlsConn,
	}, nil
}

type UTLSConn struct {
	v0net.Conn // embedded Conn
	tlsConn    *tls.UConn
}

func (uc *UTLSConn) Read(b []byte) (n int, err error) {
	return uc.tlsConn.Read(b)
}

func (uc *UTLSConn) Write(b []byte) (n int, err error) {
	return uc.tlsConn.Write(b)
}
